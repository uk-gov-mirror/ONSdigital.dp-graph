package neo4j

import (
	"context"
	"fmt"

	"github.com/ONSdigital/dp-graph/v2/graph/driver"
	"github.com/ONSdigital/dp-graph/v2/models"
	"github.com/ONSdigital/dp-graph/v2/neo4j/mapper"
	"github.com/ONSdigital/dp-graph/v2/neo4j/query"
	"github.com/ONSdigital/log.go/log"
)

// Type check to ensure that Neo4j implements the driver.CodeList interface
var _ driver.CodeList = (*Neo4j)(nil)

// GetCodeLists returns a list of code lists
func (n *Neo4j) GetCodeLists(ctx context.Context, filterBy string) (*models.CodeListResults, error) {
	logData := log.Data{}
	if len(filterBy) > 0 {
		logData["filter_by"] = filterBy
		filterBy = ":_" + filterBy
	}
	log.Event(ctx, "about to query neo4j for code lists", log.INFO, logData)

	query := fmt.Sprintf(query.GetCodeLists, filterBy)
	codeListResults := &models.CodeListResults{}

	err := n.Read(query, mapper.CodeLists(codeListResults), false)
	if err != nil {
		return nil, err
	}

	return codeListResults, nil
}

// GetCodeList returns the specified codelist
func (n *Neo4j) GetCodeList(ctx context.Context, code string) (*models.CodeList, error) {
	log.Event(ctx, "about to query neo4j for code list", log.INFO, log.Data{"code_list_id": code})

	query := fmt.Sprintf(query.GetCodeList, code)
	codeListResult := &models.CodeList{}

	if err := n.Read(query, mapper.CodeList(codeListResult, code), true); err != nil {
		//includes not found/404 responses
		return nil, err
	}

	return codeListResult, nil
}

// GetEditions returns a list of editions for a specified code list
func (n *Neo4j) GetEditions(ctx context.Context, codeListID string) (*models.Editions, error) {
	log.Event(ctx, "about to query neo4j for code list editions", log.INFO, log.Data{"code_list_id": codeListID})

	query := fmt.Sprintf(query.GetCodeList, codeListID)
	editions := &models.Editions{}

	if err := n.Read(query, mapper.Editions(editions), false); err != nil {
		return nil, err
	}

	return editions, nil
}

// GetEdition returns the specified edition for a code list
func (n *Neo4j) GetEdition(ctx context.Context, codeListID, editionID string) (*models.Edition, error) {
	log.Event(ctx, "about to query neo4j for code list edition", log.INFO, log.Data{"code_list_id": codeListID, "edition": editionID})

	query := fmt.Sprintf(query.GetCodeListEdition, codeListID, editionID)
	edition := &models.Edition{}

	if err := n.Read(query, mapper.Edition(edition), true); err != nil {
		return nil, err
	}

	return edition, nil
}

func (n *Neo4j) CountCodes(ctx context.Context, codeListID, edition string) (int64, error) {
	return 0, driver.ErrNotImplemented
}

// GetCodes returns a list of codes for a specified edition of a code list
func (n *Neo4j) GetCodes(ctx context.Context, codeListID, editionID string) (*models.CodeResults, error) {
	log.Event(ctx, "about to query neo4j for codes", log.INFO, log.Data{"code_list_id": codeListID, "edition": editionID})

	exists, err := n.GetEdition(ctx, codeListID, editionID)
	if err != nil || exists == nil {
		return nil, driver.ErrNotFound
	}

	codes := &models.CodeResults{}
	query := fmt.Sprintf(query.GetCodes, codeListID, editionID)
	if err := n.Read(query, mapper.Codes(codes, codeListID, editionID), false); err != nil {
		return nil, err
	}

	return codes, nil
}

// GetCode returns the specified code for an edition of a code list
func (n *Neo4j) GetCode(ctx context.Context, codeListID, editionID string, codeID string) (*models.Code, error) {
	log.Event(ctx, "about to query neo4j for specific code", log.INFO, log.Data{"code_list_id": codeListID, "edition": editionID, "code": codeID})

	exists, err := n.GetEdition(ctx, codeListID, editionID)
	if err != nil || exists == nil {
		return nil, driver.ErrNotFound
	}

	code := &models.Code{}
	query := fmt.Sprintf(query.GetCode, codeListID, editionID, codeID)
	if err := n.Read(query, mapper.Code(code, codeListID, editionID), true); err != nil {
		return nil, err
	}

	return code, nil
}

// GetCodesOrder is not implemented
func (n *Neo4j) GetCodesOrder(ctx context.Context, codeListID string, codes []string) (codeOrders map[string]*int, err error) {
	return nil, driver.ErrNotImplemented
}

// GetCodeDatasets returns a list of datasets where the code is used
func (n *Neo4j) GetCodeDatasets(ctx context.Context, codeListID, edition string, code string) (*models.Datasets, error) {
	log.Event(ctx, "about to query neo4j for datasets by code", log.INFO, log.Data{"code_list_id": codeListID, "edition": edition, "code": code})

	exists, err := n.GetEdition(ctx, codeListID, edition)
	if err != nil || exists == nil {
		return nil, driver.ErrNotFound
	}

	datasets := make(mapper.Datasets)
	query := fmt.Sprintf(query.GetCodeDatasets, codeListID, edition, code)
	if err := n.Read(query, mapper.CodesDatasets(datasets), false); err != nil {
		return nil, err
	}

	response := &models.Datasets{
		Items: []models.Dataset{},
	}

	for id, data := range datasets {
		dataset := models.Dataset{
			ID:             id,
			DimensionLabel: data.DimensionLabel,
		}

		for editionID, versionList := range data.Editions {
			dataset.Editions = append(dataset.Editions, models.DatasetEdition{
				ID:            editionID,
				CodeListID:    codeListID,
				LatestVersion: max(versionList),
			})
		}

		response.Items = append(response.Items, dataset)
	}

	return response, nil
}

func max(input []int) (max int) {
	for _, value := range input {
		if value > max {
			max = value
		}
	}
	return max
}
