package neptune

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/ONSdigital/dp-graph/v2/graph/driver"
	"github.com/ONSdigital/dp-graph/v2/models"
	"github.com/ONSdigital/dp-graph/v2/neptune/query"
	"github.com/ONSdigital/graphson"
	"github.com/ONSdigital/log.go/log"
)

// Type check to ensure that NeptuneDB implements the driver.Hierarchy interface
var _ driver.Hierarchy = (*NeptuneDB)(nil)

func (n *NeptuneDB) CreateInstanceHierarchyConstraints(ctx context.Context, attempt int, instanceID, dimensionName string) error {
	return nil
}

// GetCodesWithData returns a list of values that are present in nodes with label _{instanceID}_{dimensionName}
func (n *NeptuneDB) GetCodesWithData(ctx context.Context, attempt int, instanceID, dimensionName string) (codes []string, err error) {
	codesWithDataStmt := fmt.Sprintf(
		query.GetCodesWithData,
		instanceID,
		dimensionName,
	)

	logData := log.Data{
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
	}

	log.Event(ctx, "getting instance dimension codes that have data", log.INFO, logData)

	codes, err = n.getStringList(codesWithDataStmt)
	if err != nil {
		return nil, errors.Wrapf(err, "Gremlin query failed: %q", codesWithDataStmt)
	}
	return codes, nil
}

// GetGenericHierarchyNodeIDs obtains a list of node IDs for generic hierarchy nodes for the provided codeListID, which have a code in the provided list.
func (n *NeptuneDB) GetGenericHierarchyNodeIDs(ctx context.Context, attempt int, codeListID string, codes []string) (nodeIDs map[string]string, err error) {
	return n.doGetGenericHierarchyNodeIDs(ctx, attempt, codeListID, codes, false)
}

// GetGenericHierarchyAncestriesIDs obtains a list of node IDs for the parents of the hierarchy nodes that have a code in the provided list.
func (n *NeptuneDB) GetGenericHierarchyAncestriesIDs(ctx context.Context, attempt int, codeListID string, codes []string) (nodeIDs map[string]string, err error) {
	return n.doGetGenericHierarchyNodeIDs(ctx, attempt, codeListID, codes, true)
}

// clone generic hierarchy nodes in batches. This method returns unique nodeIDs (as a map, for efficiency) among all batches
func (n *NeptuneDB) doGetGenericHierarchyNodeIDs(ctx context.Context, attempt int, codeListID string, codes []string, ancestries bool) (nodeIDs map[string]string, err error) {
	logData := log.Data{
		"fn":           "GetGenericHierarchyNodeIDs",
		"code_list_id": codeListID,
		"num_codes":    len(codes),
		"max_workers":  n.maxWorkers,
		"batch_size":   n.batchSizeReader,
	}

	if ancestries {
		log.Event(ctx, "getting generic hierarchy node ancestry ids for the provided codes", log.INFO, logData)
	} else {
		log.Event(ctx, "getting generic hierarchy node ids for the provided codes", log.INFO, logData)
	}

	processBatch := func(chunkCodes map[string]string) (ret map[string]string, err error) {
		nodeIdOrders := make(map[string]string)

		codesString := `['` + strings.Join(createArray(chunkCodes), `','`) + `']`
		var stmt string
		if ancestries {
			stmt = fmt.Sprintf(
				query.GetGenericHierarchyAncestryIDs,
				codeListID,
				codesString,
			)
		} else {
			stmt = fmt.Sprintf(
				query.GetGenericHierarchyNodeIDs,
				codeListID,
				codesString,
			)
		}

		// execute query
		res, err := n.exec(stmt)
		if err != nil {
			return nil, errors.Wrapf(err, "Gremlin query failed: %q", stmt)
		}

		// responses are batched by gremgo library, hence we need to iterate them
		for _, result := range res {

			// get list of node_id to node_code maps from the response
			idCodeMap, err := graphson.DeserializeListFromBytes(result.Result.Data)
			if err != nil {
				return nil, err
			}

			// each item is a map of {'node_id': <id>, 'node_code': <code>}
			for _, val := range idCodeMap {
				nodeIdCodeMap, err := graphson.DeserializeMapFromBytes(val)
				if err != nil {
					return nil, err
				}

				nodeId, code, err := getNodeIdCodeFromMap(nodeIdCodeMap)
				if err != nil {
					return nil, err
				}
				nodeIdOrders[nodeId] = code
			}
		}
		return nodeIdOrders, nil
	}

	r, _, errs := processInConcurrentBatches(createMapFromArrays(codes), processBatch, n.batchSizeReader, n.maxWorkers)
	if len(errs) > 0 {
		return map[string]string{}, errs[0]
	}

	// convert map of interfaces to map of strings and return
	return r, nil
}

func getNodeIdCodeFromMap(nodeCodeMap map[string]json.RawMessage) (nodeID string, code string, err error) {
	rawNodeId, ok := nodeCodeMap["node_id"]
	if !ok {
		return "", "", driver.ErrNotFound
	}

	rawCode, ok := nodeCodeMap["node_code"]
	if !ok {
		return "", "", driver.ErrNotFound
	}

	if err := json.Unmarshal(rawNodeId, &nodeID); err != nil {
		return "", "", err
	}

	if err := json.Unmarshal(rawCode, &code); err != nil {
		return "", "", err
	}

	return nodeID, code, nil
}

func (n *NeptuneDB) CreateHasCodeEdges(ctx context.Context, attempt int, codeListID string, codesById map[string]string) (err error) {
	logData := log.Data{
		"fn":           "CreateHasCodeEdges",
		"code_list_id": codeListID,
		"num_codes":    len(codesById),
		"max_workers":  n.maxWorkers,
		"batch_size":   n.batchSizeReader,
	}
	log.Event(ctx, "creating 'hasCode' edges between generic hierarchy nodes and their corresponding code nodes", log.INFO, logData)

	// although we expect a size of one, we leave the logic to perform multiple sequential operaions per batch processor for completeness
	processBatch := func(chunk map[string]string) (ret map[string]string, err error) {
		for nodeId, code := range chunk {
			stmt := fmt.Sprintf(query.CreateHasCodeEdge, code, codeListID, nodeId)
			if _, err := n.exec(stmt); err != nil {
				return nil, errors.Wrapf(err, "Gremlin query failed: %q", stmt)
			}
		}
		return nil, nil
	}

	_, _, errs := processInConcurrentBatches(codesById, processBatch, 1, n.maxWorkers)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (n *NeptuneDB) CloneNodes(ctx context.Context, attempt int, instanceID, codeListID, dimensionName string) (err error) {
	gremStmt := fmt.Sprintf(
		query.CloneHierarchyNodes,
		codeListID,
		instanceID,
		dimensionName,
		codeListID,
	)
	logData := log.Data{"fn": "CloneNodes",
		"gremlin":        gremStmt,
		"instance_id":    instanceID,
		"code_list_id":   codeListID,
		"dimension_name": dimensionName,
	}
	log.Event(ctx, "cloning all nodes from the generic hierarchy", log.INFO, logData)

	if _, err = n.exec(gremStmt); err != nil {
		log.Event(ctx, "cannot get vertices during cloning", log.ERROR, logData, log.Error(err))
		return
	}

	return
}

// CloneNodesFromIDs clones the generic hierarchy nodes with the provided IDs (map for uniqueness and efficiency)
func (n *NeptuneDB) CloneNodesFromIDs(ctx context.Context, attempt int, instanceID, codeListID, dimensionName string, ids map[string]string, hasData bool) (err error) {
	logData := log.Data{"fn": "CloneNodesFromIDs",
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
		"code_list_id":   codeListID,
		"has_data":       hasData,
		"num_nodes":      len(ids),
		"max_workers":    n.maxWorkers,
		"batch_size":     n.batchSizeWriter,
	}
	log.Event(ctx, "cloning necessary nodes from the generic hierarchy", log.INFO, logData)

	processBatch := func(chunkIDs map[string]string) (ret map[string]string, err error) {
		idsStr := `'` + strings.Join(createArray(chunkIDs), `','`) + `'`
		gremStmt := fmt.Sprintf(
			query.CloneHierarchyNodesFromIDs,
			idsStr,
			instanceID,
			dimensionName,
			hasData,
			codeListID,
		)

		if _, err = n.exec(gremStmt); err != nil {
			log.Event(ctx, "cannot get vertices during cloning", log.ERROR, logData, log.Error(err))
			return nil, err
		}
		return nil, nil
	}

	_, _, errs := processInConcurrentBatches(ids, processBatch, n.batchSizeWriter, n.maxWorkers)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// CloneOrderFromIDs copies the order property from the 'usedBy' edge that goes from the code node to the provided codelist node
// where the code node is the determined by the 'hasCode' edge of the generic hierarchy nodes.
// The order property is stored as a property of the clone node (assumes a clone_of edge exists from a hierarchy node to the generic hierarchy node)
func (n *NeptuneDB) CloneOrderFromIDs(ctx context.Context, codeListID string, ids map[string]string) (err error) {
	logData := log.Data{"fn": "CloneOrderFromIDs",
		"code_list_id": codeListID,
		"num_nodes":    len(ids),
		"max_workers":  n.maxWorkers,
		"batch_size":   n.batchSizeWriter,
	}
	log.Event(ctx, "cloning order property corresponding to the code of the generic hierarchy nodes", log.INFO, logData)

	processBatch := func(chunkIDs map[string]string) (ret map[string]string, err error) {
		idsStr := `'` + strings.Join(createArray(chunkIDs), `','`) + `'`
		gremStmt := fmt.Sprintf(
			query.CloneOrderFromIDs,
			idsStr,
			codeListID,
		)

		if _, err = n.exec(gremStmt); err != nil {
			log.Event(ctx, "cannot get vertices during cloning", log.ERROR, logData, log.Error(err))
			return nil, err
		}
		return nil, nil
	}

	_, _, errs := processInConcurrentBatches(ids, processBatch, n.batchSizeWriter, n.maxWorkers)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// CountNodes returns the number of hierarchy nodes for the provided instanceID and dimensionName
func (n *NeptuneDB) CountNodes(ctx context.Context, instanceID, dimensionName string) (count int64, err error) {
	gremStmt := fmt.Sprintf(query.CountHierarchyNodes, instanceID, dimensionName)
	logData := log.Data{
		"fn":             "CountNodes",
		"gremlin":        gremStmt,
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
	}
	log.Event(ctx, "counting nodes in the new instance hierarchy", log.INFO, logData)

	if count, err = n.getNumber(gremStmt); err != nil {
		log.Event(ctx, "cannot count nodes in a hierarchy", log.ERROR, logData, log.Error(err))
		return
	}
	return
}

func (n *NeptuneDB) CloneRelationships(ctx context.Context, attempt int, instanceID, codeListID, dimensionName string) (err error) {
	gremStmt := fmt.Sprintf(
		query.CloneHierarchyRelationships,
		codeListID,
		instanceID,
		dimensionName,
		instanceID,
		dimensionName,
	)

	logData := log.Data{
		"fn":             "CloneRelationships",
		"instance_id":    instanceID,
		"code_list_id":   codeListID,
		"dimension_name": dimensionName,
		"gremlin":        gremStmt,
	}
	log.Event(ctx, "cloning relationships from the generic hierarchy", log.INFO, logData)

	if _, err = n.getEdges(gremStmt); err != nil {
		log.Event(ctx, "cannot find edges while cloning relationships", log.ERROR, logData, log.Error(err))
		return
	}

	return n.RemoveCloneEdges(ctx, attempt, instanceID, dimensionName)
}

// CloneRelationshipsFromIDs clones the has_parent edges between clones that have parent relationship according to the provided generic hierarchy nodes.
func (n *NeptuneDB) CloneRelationshipsFromIDs(ctx context.Context, attempt int, instanceID, dimensionName string, ids map[string]string) error {
	logData := log.Data{
		"fn":             "CloneRelationshipsFromIDs",
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
		"num_ids":        len(ids),
		"max_workers":    n.maxWorkers,
		"batch_size":     n.batchSizeWriter,
	}
	log.Event(ctx, "cloning relationships from the generic hierarchy", log.INFO, logData)

	processBatch := func(chunkIDs map[string]string) (ret map[string]string, err error) {
		idsStr := `'` + strings.Join(createArray(chunkIDs), `','`) + `'`
		gremStmt := fmt.Sprintf(
			query.CloneHierarchyRelationshipsFromIDs,
			idsStr,
			instanceID,
			dimensionName,
			instanceID,
			dimensionName,
		)

		if _, err := n.getEdges(gremStmt); err != nil {
			log.Event(ctx, "cannot find edges while cloning relationships", log.ERROR, logData, log.Error(err))
			return nil, err
		}
		return nil, nil
	}

	_, _, errs := processInConcurrentBatches(ids, processBatch, n.batchSizeWriter, n.maxWorkers)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// GetHierarchyNodeIDs returns a map of IDs for the cloned hierarchy nodes for a provided instanceID and dimensionName
func (n *NeptuneDB) GetHierarchyNodeIDs(ctx context.Context, attempt int, instanceID, dimensionName string) (ids map[string]string, err error) {
	stmt := fmt.Sprintf(
		query.GetHierarchyNodeIDs,
		instanceID,
		dimensionName,
	)
	logData := log.Data{
		"fn":             "GetHierarchyNodeIDs",
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
		"gremlin":        stmt,
	}
	log.Event(ctx, "getting ids of cloned hierarchy nodes", log.INFO, logData)

	idList, err := n.getStringList(stmt)
	if err != nil {
		return nil, errors.Wrapf(err, "Gremlin query failed: %q", stmt)
	}
	return createStringMapFromArrays(idList), nil
}

func (n *NeptuneDB) RemoveCloneEdges(ctx context.Context, attempt int, instanceID, dimensionName string) (err error) {
	gremStmt := fmt.Sprintf(
		query.RemoveCloneMarkers,
		instanceID,
		dimensionName,
	)
	logData := log.Data{
		"fn":             "RemoveCloneEdges",
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
		"gremlin":        gremStmt,
	}
	log.Event(ctx, "removing edges to generic hierarchy", log.INFO, logData)

	if _, err = n.exec(gremStmt); err != nil {
		log.Event(ctx, "exec failed while removing edges during removal of unwanted cloned edges", log.ERROR, logData, log.Error(err))
		return
	}
	return
}

// RemoveCloneEdgesFromSourceIDs removes the 'clone-of' edges between a set of cloned nodes and their corresponding generic hierarchy nodes.
func (n *NeptuneDB) RemoveCloneEdgesFromSourceIDs(ctx context.Context, attempt int, ids map[string]string) (err error) {
	logData := log.Data{
		"fn":          "RemoveCloneEdges",
		"num_ids":     len(ids),
		"max_workers": n.maxWorkers,
		"batch_size":  n.batchSizeWriter,
	}
	log.Event(ctx, "removing edges to generic hierarchy", log.INFO, logData)

	processBatch := func(chunkIDs map[string]string) (ret map[string]string, err error) {
		idsStr := `'` + strings.Join(createArray(chunkIDs), `','`) + `'`
		gremStmt := fmt.Sprintf(
			query.RemoveCloneMarkersFromSourceIDs,
			idsStr,
		)

		if _, err = n.exec(gremStmt); err != nil {
			log.Event(ctx, "exec failed while removing edges during removal of unwanted cloned edges", log.ERROR, logData, log.Error(err))
			return
		}
		return
	}

	_, _, errs := processInConcurrentBatches(ids, processBatch, n.batchSizeWriter, n.maxWorkers)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (n *NeptuneDB) SetNumberOfChildren(ctx context.Context, attempt int, instanceID, dimensionName string) (err error) {
	gremStmt := fmt.Sprintf(
		query.SetNumberOfChildren,
		instanceID,
		dimensionName,
	)

	logData := log.Data{
		"fn":             "SetNumberOfChildren",
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
		"gremlin":        gremStmt,
	}

	log.Event(ctx, "setting number-of-children property value on the instance hierarchy nodes", log.INFO, logData)

	if _, err = n.exec(gremStmt); err != nil {
		log.Event(ctx, "cannot find vertices while setting nChildren on hierarchy nodes", log.ERROR, logData, log.Error(err))
		return
	}

	return
}

// SetNumberOfChildrenFromIDs sets a property called 'numberOfChildren' to the value indegree of edges 'hasParent' for the provided node IDs
func (n *NeptuneDB) SetNumberOfChildrenFromIDs(ctx context.Context, attempt int, ids map[string]string) (err error) {
	logData := log.Data{
		"fn":          "SetNumberOfChildren",
		"num_ids":     len(ids),
		"max_workers": n.maxWorkers,
		"batch_size":  n.batchSizeWriter,
	}
	log.Event(ctx, "setting number-of-children property value on the instance hierarchy nodes", log.INFO, logData)

	processBatch := func(chunkIDs map[string]string) (ret map[string]string, err error) {
		idsStr := `'` + strings.Join(createArray(chunkIDs), `','`) + `'`
		gremStmt := fmt.Sprintf(
			query.SetNumberOfChildrenFromIDs,
			idsStr,
		)

		if _, err = n.exec(gremStmt); err != nil {
			log.Event(ctx, "cannot find vertices while setting nChildren on hierarchy nodes", log.ERROR, logData, log.Error(err))
			return
		}
		return
	}

	_, _, errs := processInConcurrentBatches(ids, processBatch, n.batchSizeWriter, n.maxWorkers)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (n *NeptuneDB) SetHasData(ctx context.Context, attempt int, instanceID, dimensionName string) (err error) {

	codesWithDataStmt := fmt.Sprintf(
		query.GetCodesWithData,
		instanceID,
		dimensionName,
	)

	logData := log.Data{
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
	}

	log.Event(ctx, "getting instance dimension codes that have data", log.INFO, logData)

	codes, err := n.getStringList(codesWithDataStmt)
	if err != nil {
		return errors.Wrapf(err, "Gremlin query failed: %q", codesWithDataStmt)
	}

	codesString := `['` + strings.Join(codes, `','`) + `']`

	gremStmt := fmt.Sprintf(
		query.SetHasData,
		instanceID,
		dimensionName,
		codesString,
	)

	log.Event(ctx, "setting has-data property on the instance hierarchy", log.INFO, logData)

	if _, err = n.exec(gremStmt); err != nil {
		log.Event(ctx, "cannot find vertices while setting hasData on hierarchy nodes", log.ERROR, logData, log.Error(err))
		return
	}

	return
}

func (n *NeptuneDB) MarkNodesToRemain(ctx context.Context, attempt int, instanceID, dimensionName string) (err error) {
	gremStmt := fmt.Sprintf(query.MarkNodesToRemain,
		instanceID,
		dimensionName,
	)

	logData := log.Data{
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
		"gremlin":        gremStmt,
	}

	log.Event(ctx, "marking nodes to remain after trimming sparse branches", log.INFO, logData)

	if _, err = n.exec(gremStmt); err != nil {
		log.Event(ctx, "cannot find vertices while marking hierarchy nodes to keep", log.ERROR, logData, log.Error(err))
		return
	}

	return
}

func (n *NeptuneDB) RemoveNodesNotMarkedToRemain(ctx context.Context, attempt int, instanceID, dimensionName string) (err error) {
	gremStmt := fmt.Sprintf(query.RemoveNodesNotMarkedToRemain, instanceID, dimensionName)
	logData := log.Data{
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
		"gremlin":        gremStmt,
	}

	log.Event(ctx, "removing nodes not marked to remain after trimming sparse branches", log.INFO, logData)

	if _, err = n.exec(gremStmt); err != nil {
		log.Event(ctx, "exec query failed while removing hierarchy nodes to cull", log.ERROR, logData, log.Error(err))
		return
	}
	return
}

func (n *NeptuneDB) RemoveRemainMarker(ctx context.Context, attempt int, instanceID, dimensionName string) (err error) {
	gremStmt := fmt.Sprintf(query.RemoveRemainMarker, instanceID, dimensionName)
	logData := log.Data{
		"fn":             "RemoveRemainMarker",
		"gremlin":        gremStmt,
		"instance_id":    instanceID,
		"dimension_name": dimensionName,
	}
	log.Event(ctx, "removing the remain property from the nodes that remain", log.INFO, logData)

	if _, err = n.exec(gremStmt); err != nil {
		log.Event(ctx, "exec query failed while removing spent remain markers from hierarchy nodes", log.ERROR, logData, log.Error(err))
		return
	}
	return
}

func (n *NeptuneDB) GetHierarchyCodelist(ctx context.Context, instanceID, dimension string) (codelistID string, err error) {
	gremStmt := fmt.Sprintf(query.HierarchyExists, instanceID, dimension)
	logData := log.Data{
		"fn":             "GetHierarchyCodelist",
		"gremlin":        gremStmt,
		"instance_id":    instanceID,
		"dimension_name": dimension,
	}

	var vertex graphson.Vertex
	if vertex, err = n.getVertex(gremStmt); err != nil {
		log.Event(ctx, "cannot get vertices  while searching for code list node related to hierarchy node", log.ERROR, logData, log.Error(err))
		return
	}
	if codelistID, err = vertex.GetProperty("code_list"); err != nil {
		log.Event(ctx, "cannot read code_list property from node", log.ERROR, logData, log.Error(err))
		return
	}
	return
}

func (n *NeptuneDB) GetHierarchyRoot(ctx context.Context, instanceID, dimension string) (node *models.HierarchyResponse, err error) {
	gremStmt := fmt.Sprintf(query.GetHierarchyRoot, instanceID, dimension)
	logData := log.Data{
		"fn":             "GetHierarchyRoot",
		"gremlin":        gremStmt,
		"instance_id":    instanceID,
		"dimension_name": dimension,
	}

	var vertices []graphson.Vertex
	if vertices, err = n.getVertices(gremStmt); err != nil {
		log.Event(ctx, "getVertices failed: cannot find hierarchy root node candidates ", log.ERROR, logData, log.Error(err))
		return
	}
	if len(vertices) == 0 {
		err = driver.ErrNotFound
		log.Event(ctx, "Cannot find hierarchy root node", log.ERROR, logData, log.Error(err))
		return
	}
	if len(vertices) > 1 {
		err = driver.ErrMultipleFound
		log.Event(ctx, "Cannot identify hierarchy root node because are multiple candidates", log.ERROR, logData, log.Error(err))
		return
	}
	vertex := vertices[0]
	// Note the call to buildHierarchyNode below does much more than meets the eye,
	// including launching new queries in of itself to fetch child nodes, and
	// breadcrumb nodes.
	wantBreadcrumbs := false // Because meaningless for a root node
	if node, err = n.buildHierarchyNode(vertex, instanceID, dimension, wantBreadcrumbs); err != nil {
		log.Event(ctx, "Cannot extract related information needed from hierarchy node", log.ERROR, logData, log.Error(err))
		return
	}
	return
}

func (n *NeptuneDB) HierarchyExists(ctx context.Context, instanceID, dimension string) (hierarchyExists bool, err error) {
	gremStmt := fmt.Sprintf(query.HierarchyExists, instanceID, dimension)
	logData := log.Data{
		"fn":             "HierarchyExists",
		"gremlin":        gremStmt,
		"instance_id":    instanceID,
		"dimension_name": dimension,
	}

	var vertices []graphson.Vertex
	if vertices, err = n.getVertices(gremStmt); err != nil {
		log.Event(ctx, "getVertices failed when attempting to get a hierarchy node", log.ERROR, logData, log.Error(err))
		return
	}

	if len(vertices) == 1 {
		hierarchyExists = true
		return hierarchyExists, nil
	}

	if len(vertices) > 1 {
		hierarchyExists = true
		err = driver.ErrMultipleFound
		log.Event(ctx, "expected a single hierarchy node but multiple were returned", log.ERROR, logData, log.Error(err))
		return hierarchyExists, err
	}

	return hierarchyExists, nil
}

func (n *NeptuneDB) GetHierarchyElement(ctx context.Context, instanceID, dimension, code string) (node *models.HierarchyResponse, err error) {
	gremStmt := fmt.Sprintf(query.GetHierarchyElement, instanceID, dimension, code)
	logData := log.Data{
		"fn":             "GetHierarchyElement",
		"gremlin":        gremStmt,
		"instance_id":    instanceID,
		"code_list_id":   code,
		"dimension_name": dimension,
	}

	var vertex graphson.Vertex
	if vertex, err = n.getVertex(gremStmt); err != nil {
		log.Event(ctx, "Cannot find vertex when looking for specific hierarchy node", log.ERROR, logData, log.Error(err))
		return
	}
	// Note the call to buildHierarchyNode below does much more than meets the eye,
	// including launching new queries in of itself to fetch child nodes, and
	// breadcrumb nodes.
	wantBreadcrumbs := true // Because we are at depth in the hierarchy
	if node, err = n.buildHierarchyNode(vertex, instanceID, dimension, wantBreadcrumbs); err != nil {
		log.Event(ctx, "Cannot extract related information needed from hierarchy node", log.ERROR, logData, log.Error(err))
		return
	}
	return
}
