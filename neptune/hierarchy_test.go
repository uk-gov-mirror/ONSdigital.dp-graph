package neptune

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ONSdigital/dp-graph/v2/graph/driver"
	"github.com/ONSdigital/dp-graph/v2/neptune/internal"
	"github.com/ONSdigital/dp-graph/v2/neptune/query"
	"github.com/ONSdigital/graphson"
	"github.com/ONSdigital/gremgo-neptune"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	ctx               = context.Background()
	testCodeListID    = "cpih1dim1aggid"
	testInstanceID    = "f0a2f3f2-cc86-4bbb-a549-ffc99c89292c"
	testDimensionName = "aggregate"
	testAttempt       = 1
	testCodes         = []string{"cpih1dim1S90401", "cpih1dim1S90402"}
	testIds           = map[string]struct{}{
		"cpih1dim1aggid--cpih1dim1S90401": {},
		"cpih1dim1aggid--cpih1dim1S90402": {}}
	testAllIds = map[string]struct{}{
		"cpih1dim1aggid--cpih1dim1S90401": {},
		"cpih1dim1aggid--cpih1dim1S90402": {},
		"cpih1dim1aggid--cpih1dim1G90400": {},
		"cpih1dim1aggid--cpih1dim1T90000": {},
		"cpih1dim1aggid--cpih1dim1A0":     {}}
	testClonedIds = map[string]struct{}{
		"62bab579-e923-7cb2-3be0-34d09dc0567b": {},
		"acbab579-e923-87df-e59a-9daf2ffed388": {},
		"b6bab57a-604d-8a7f-59f5-1d496c9b3ca5": {},
		"08bab57a-604d-9cd9-492f-e879cee05502": {},
		"6cbab57a-604d-f176-9370-c60c19369801": {},
	}
)

func TestNeptuneDB_GetCodesWithData(t *testing.T) {

	Convey("Given a neptune DB that returns a code list", t, func() {
		poolMock := &internal.NeptunePoolMock{
			GetStringListFunc: internal.ReturnCodesList,
		}
		db := mockDB(poolMock)

		Convey("When GetCodesWithData is called", func() {
			codes, err := db.GetCodesWithData(ctx, testAttempt, testInstanceID, testDimensionName)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected list of codes is returned", func() {
				So(len(codes), ShouldEqual, 2)
				So(codes, ShouldContain, "cpih1dim1S90401")
				So(codes, ShouldContain, "cpih1dim1S90402")
				expectedQuery := `g.V().hasLabel('_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate').values('value')`
				So(len(poolMock.GetStringListCalls()), ShouldEqual, 1)
				So(poolMock.GetStringListCalls()[0].Query, ShouldEqual, expectedQuery)
			})
		})
	})
}

func TestNeptuneDB_HierarchyExists(t *testing.T) {

	vertex := internal.MakeHierarchyVertex("vertex-label", "code", "label", 1, true)

	Convey("Given a neptune DB that returns a single hierarchy node", t, func() {

		poolMock := &internal.NeptunePoolMock{GetFunc: func(query string, bindings map[string]string, rebindings map[string]string) (vertices []graphson.Vertex, err error) {
			return []graphson.Vertex{vertex}, nil
		}}
		db := mockDB(poolMock)

		Convey("When HierarchyExists is called", func() {
			hierarchyExists, err := db.HierarchyExists(ctx, testInstanceID, testDimensionName)

			Convey("Then the expected query is sent to Neptune", func() {

				expectedQuery := fmt.Sprintf(query.HierarchyExists, testInstanceID, testDimensionName)
				So(len(poolMock.GetCalls()), ShouldEqual, 1)
				So(poolMock.GetCalls()[0].Query, ShouldEqual, expectedQuery)
			})

			Convey("Then the return value is true", func() {
				So(hierarchyExists, ShouldBeTrue)
				So(err, ShouldBeNil)
			})
		})
	})

	Convey("Given a neptune DB that returns multiple hierarchy nodes", t, func() {

		poolMock := &internal.NeptunePoolMock{GetFunc: func(query string, bindings map[string]string, rebindings map[string]string) (vertices []graphson.Vertex, err error) {
			return []graphson.Vertex{vertex, vertex}, nil
		}}
		db := mockDB(poolMock)

		Convey("When HierarchyExists is called", func() {
			hierarchyExists, err := db.HierarchyExists(ctx, testInstanceID, testDimensionName)

			Convey("Then the expected query is sent to Neptune", func() {

				expectedQuery := fmt.Sprintf(query.HierarchyExists, testInstanceID, testDimensionName)
				So(len(poolMock.GetCalls()), ShouldEqual, 1)
				So(poolMock.GetCalls()[0].Query, ShouldEqual, expectedQuery)
			})

			Convey("Then the return value should be true", func() {
				So(hierarchyExists, ShouldBeTrue)
			})

			Convey("Then the expected error is returned", func() {
				So(err, ShouldEqual, driver.ErrMultipleFound)
			})
		})
	})

	Convey("Given a neptune DB that returns an empty array of vertices", t, func() {

		poolMock := &internal.NeptunePoolMock{GetFunc: func(query string, bindings map[string]string, rebindings map[string]string) (vertices []graphson.Vertex, err error) {
			return []graphson.Vertex{}, nil
		}}
		db := mockDB(poolMock)

		Convey("When HierarchyExists is called", func() {
			hierarchyExists, err := db.HierarchyExists(ctx, testInstanceID, testDimensionName)

			Convey("Then the expected query is sent to Neptune", func() {
				expectedQuery := fmt.Sprintf(query.HierarchyExists, testInstanceID, testDimensionName)
				So(len(poolMock.GetCalls()), ShouldEqual, 1)
				So(poolMock.GetCalls()[0].Query, ShouldEqual, expectedQuery)
			})

			Convey("Then the return value is false", func() {
				So(hierarchyExists, ShouldBeFalse)
				So(err, ShouldBeNil)
			})
		})
	})

	Convey("Given a neptune DB that returns an error", t, func() {

		poolMock := &internal.NeptunePoolMock{
			GetFunc: internal.ReturnMalformedNilInterfaceRequestErr,
		}
		db := mockDB(poolMock)

		Convey("When HierarchyExists is called", func() {
			hierarchyExists, err := db.HierarchyExists(ctx, testInstanceID, testDimensionName)

			Convey("Then the expected query is sent to Neptune", func() {
				expectedQuery := fmt.Sprintf(query.HierarchyExists, testInstanceID, testDimensionName)
				So(len(poolMock.GetCalls()), ShouldEqual, 1)
				So(poolMock.GetCalls()[0].Query, ShouldEqual, expectedQuery)
			})

			Convey("Then the return value should be false", func() {
				So(hierarchyExists, ShouldBeFalse)
			})

			Convey("Then the expected error is returned", func() {
				So(err, ShouldEqual, internal.NonTransientErr)
			})
		})
	})
}

func mockGremgoResponseNodeIDCodeMap(expectedNodeIdCodeMap map[string]string) []gremgo.Response {
	values := []json.RawMessage{}
	for nodeId, code := range expectedNodeIdCodeMap {
		rawMap := mockNodeIdCodeMapResponse(nodeId, code)
		values = append(values, rawMap)
	}

	testData := graphson.RawSlice{
		Type:  "g:List",
		Value: values,
	}
	rawTestData, err := json.Marshal(testData)
	So(err, ShouldBeNil)

	return []gremgo.Response{
		{
			RequestID: "89ed2475-6eb8-452b-a955-7f7697de2ff9",
			Status:    gremgo.Status{Message: "", Code: 200},
			Result: gremgo.Result{
				Data: rawTestData,
			},
		},
	}
}

func TestNeptuneDB_GetGenericHierarchyNodeIDs(t *testing.T) {

	Convey("Given a neptune DB that returns a list of generic hierarchy node IDs (leaves)", t, func() {
		expectedResponse := map[string]string{
			"cpih1dim1aggid--cpih1dim1S90401": "cpih1dim1S90401",
			"cpih1dim1aggid--cpih1dim1S90402": "cpih1dim1S90402",
		}
		gremgoResponse := mockGremgoResponseNodeIDCodeMap(expectedResponse)
		poolMock := &internal.NeptunePoolMock{
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) ([]gremgo.Response, error) {
				return gremgoResponse, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When GetGenericHierarchyNodeIDs is called with a list of codes", func() {
			ids, err := db.GetGenericHierarchyNodeIDs(ctx, testAttempt, testCodeListID, testCodes)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected list of IDs is returned and the expected query is executed, in any order of IDs", func() {
				So(ids, ShouldResemble, expectedResponse)
				expectedQueryOp1 := `g.V().hasLabel('_generic_hierarchy_node_cpih1dim1aggid').has('code',within(['cpih1dim1S90401','cpih1dim1S90402'])).as('gh').id().as('node_id').select('gh').values('code').as('node_code').select('gh').select('node_id', 'node_code')`
				expectedQueryOp2 := `g.V().hasLabel('_generic_hierarchy_node_cpih1dim1aggid').has('code',within(['cpih1dim1S90402','cpih1dim1S90401'])).as('gh').id().as('node_id').select('gh').values('code').as('node_code').select('gh').select('node_id', 'node_code')`
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 1)
				So(poolMock.ExecuteCalls()[0].Query, ShouldBeIn, []string{expectedQueryOp1, expectedQueryOp2})
			})
		})

		Convey("When GetGenericHierarchyNodeIDs is called with an empty list of codes", func() {
			ids, err := db.GetGenericHierarchyNodeIDs(ctx, testAttempt, testCodeListID, []string{})

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then an empty map of IDs is returned and no query is executed", func() {
				So(ids, ShouldResemble, map[string]string{})
				So(len(poolMock.GetStringListCalls()), ShouldEqual, 0)
			})
		})
	})
}

func TestNeptuneDB_GetGenericHierarchyAncestriesIDs(t *testing.T) {

	Convey("Given a neptune DB that returns a list of generic ancestry hierarchy node IDs, with duplicates", t, func() {
		expectedResponse := map[string]string{
			"cpih1dim1aggid--cpih1dim1G90400": "cpih1dim1G90400",
			"cpih1dim1aggid--cpih1dim1T90000": "cpih1dim1T90000",
			"cpih1dim1aggid--cpih1dim1A0":     "cpih1dim1A0",
		}
		gremgoResponse := mockGremgoResponseNodeIDCodeMap(expectedResponse)
		poolMock := &internal.NeptunePoolMock{
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) ([]gremgo.Response, error) {
				return gremgoResponse, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When GetGenericHierarchyAncestriesIDs is called with a list of codes", func() {
			ids, err := db.GetGenericHierarchyAncestriesIDs(ctx, testAttempt, testCodeListID, testCodes)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected list of unique IDs is returned and teh expected is executed, in any order of IDs", func() {
				So(ids, ShouldResemble, expectedResponse)
				expectedQueryOp1 := `g.V().hasLabel('_generic_hierarchy_node_cpih1dim1aggid').has('code',within(['cpih1dim1S90401','cpih1dim1S90402'])).repeat(out('hasParent')).emit().as('gh')` +
					`.id().as('node_id').select('gh').values('code').as('node_code').select('gh').select('node_id', 'node_code')`
				expectedQueryOp2 := `g.V().hasLabel('_generic_hierarchy_node_cpih1dim1aggid').has('code',within(['cpih1dim1S90402','cpih1dim1S90401'])).repeat(out('hasParent')).emit().as('gh')` +
					`.id().as('node_id').select('gh').values('code').as('node_code').select('gh').select('node_id', 'node_code')`
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 1)
				So(poolMock.ExecuteCalls()[0].Query, ShouldBeIn, []string{expectedQueryOp1, expectedQueryOp2})
			})
		})

		Convey("When GetGenericHierarchyAncestriesIDs is called with an empty list of codes", func() {
			ids, err := db.GetGenericHierarchyAncestriesIDs(ctx, testAttempt, testCodeListID, []string{})

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then an empty map of IDs is returned and no query is executed", func() {
				So(ids, ShouldResemble, map[string]string{})
				So(len(poolMock.GetStringListCalls()), ShouldEqual, 0)
			})
		})
	})
}

func TestNeptuneDB_CloneNodesFromID(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		poolMock := &internal.NeptunePoolMock{
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) (responses []gremgo.Response, err error) {
				return []gremgo.Response{}, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When CloneNodes is called with a map of IDs", func() {
			err := db.CloneNodesFromIDs(ctx, testAttempt, testInstanceID, testCodeListID, testDimensionName, testIds, true)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected query is sent to  Neptune to clone the nodes with the provided ids", func() {
				expectedQueryFmt := `g.V('%s','%s').as('old')` +
					`.addV('_hierarchy_node_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate')` +
					`.property(single,'code',select('old').values('code'))` +
					`.property(single,'label',select('old').values('label'))` +
					`.property(single,'hasData', true)` +
					`.property('code_list','cpih1dim1aggid').as('new')` +
					`.addE('clone_of').to('old')`
				expectedQueryOp1 := fmt.Sprintf(expectedQueryFmt, "cpih1dim1aggid--cpih1dim1S90401", "cpih1dim1aggid--cpih1dim1S90402")
				expectedQueryOp2 := fmt.Sprintf(expectedQueryFmt, "cpih1dim1aggid--cpih1dim1S90402", "cpih1dim1aggid--cpih1dim1S90401")
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 1)
				So(poolMock.ExecuteCalls()[0].Query, ShouldBeIn, []string{expectedQueryOp1, expectedQueryOp2})
			})
		})

		Convey("When CloneNodes is called with an empty map of IDs", func() {
			err := db.CloneNodesFromIDs(ctx, testAttempt, testInstanceID, testCodeListID, testDimensionName, map[string]struct{}{}, true)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then no query is executed", func() {
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 0)
			})
		})
	})
}

func TestNeptuneDB_CountNodes(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		var expectedCount int64 = 123
		poolMock := &internal.NeptunePoolMock{
			GetCountFunc: func(q string, bindings map[string]string, rebindings map[string]string) (int64, error) {
				return expectedCount, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When CountNodes is called", func() {
			count, err := db.CountNodes(ctx, testInstanceID, testDimensionName)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected query is sent to Neptune and the expected count is returned", func() {
				So(count, ShouldEqual, expectedCount)
				expectedQuery := `g.V().hasLabel('_hierarchy_node_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate').count()`
				So(len(poolMock.GetCountCalls()), ShouldEqual, 1)
				So(poolMock.GetCountCalls()[0].Q, ShouldEqual, expectedQuery)
			})
		})
	})
}

func TestNeptuneDB_CloneRelationshipsFromIDs(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		poolMock := &internal.NeptunePoolMock{
			GetEFunc: func(q string, bindings, rebindings map[string]string) (resp interface{}, err error) {
				return []graphson.Edge{}, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When CloneRelationShips is called with a map of IDs", func() {
			err := db.CloneRelationshipsFromIDs(ctx, testAttempt, testInstanceID, testDimensionName, testAllIds)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected query is sent to Neptune to clone the nodes with the unique provided IDs in any order", func() {
				expectedQPrefix := `g.V('`
				expectedQSuffix := `').as('oc')` +
					`.out('hasParent')` +
					`.in('clone_of').hasLabel('_hierarchy_node_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate').as('p')` +
					`.select('oc').in('clone_of').hasLabel('_hierarchy_node_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate')` +
					`.addE('hasParent').to('p')`
				So(len(poolMock.GetECalls()), ShouldEqual, 1)
				So(strings.HasPrefix(poolMock.GetECalls()[0].Q, expectedQPrefix), ShouldBeTrue)
				So(strings.Count(poolMock.GetECalls()[0].Q, "'cpih1dim1aggid--cpih1dim1S90401'"), ShouldEqual, 1)
				So(strings.Count(poolMock.GetECalls()[0].Q, "'cpih1dim1aggid--cpih1dim1S90402'"), ShouldEqual, 1)
				So(strings.Count(poolMock.GetECalls()[0].Q, "'cpih1dim1aggid--cpih1dim1G90400'"), ShouldEqual, 1)
				So(strings.Count(poolMock.GetECalls()[0].Q, "'cpih1dim1aggid--cpih1dim1T90000'"), ShouldEqual, 1)
				So(strings.Count(poolMock.GetECalls()[0].Q, "'cpih1dim1aggid--cpih1dim1A0'"), ShouldEqual, 1)
				So(strings.HasSuffix(poolMock.GetECalls()[0].Q, expectedQSuffix), ShouldBeTrue)
			})
		})

		Convey("When CloneRelationShips is called with an empty map of IDs", func() {
			err := db.CloneRelationshipsFromIDs(ctx, testAttempt, testInstanceID, testDimensionName, map[string]struct{}{})

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then no query is executed", func() {
				So(len(poolMock.GetECalls()), ShouldEqual, 0)
			})
		})
	})
}

func TestNeptuneDB_RemoveCloneEdges(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		poolMock := &internal.NeptunePoolMock{
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) (responses []gremgo.Response, err error) {
				return []gremgo.Response{}, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When RemoveCloneEdges is called", func() {
			err := db.RemoveCloneEdges(ctx, testAttempt, testInstanceID, testDimensionName)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the clone relationships are removed", func() {
				expectedQuery := `g.V().hasLabel('_hierarchy_node_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate').outE('clone_of').drop()`
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 1)
				So(poolMock.ExecuteCalls()[0].Query, ShouldEqual, expectedQuery)
			})
		})
	})
}

func TestNeptuneDB_RemoveCloneEdgesFromSourceIDs(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		poolMock := &internal.NeptunePoolMock{
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) (responses []gremgo.Response, err error) {
				return []gremgo.Response{}, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When RemoveCloneEdgesFromSourceIDs is called with a map of IDs", func() {
			err := db.RemoveCloneEdgesFromSourceIDs(ctx, testAttempt, testClonedIds)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the clone relationships are removed", func() {
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 1)
				expectedQPrefix := `g.V('`
				expectedQSuffix := `').outE('clone_of').drop()`
				So(strings.HasPrefix(poolMock.ExecuteCalls()[0].Query, expectedQPrefix), ShouldBeTrue)
				for id := range testClonedIds {
					So(strings.Count(poolMock.ExecuteCalls()[0].Query, id), ShouldEqual, 1)
				}
				So(strings.HasSuffix(poolMock.ExecuteCalls()[0].Query, expectedQSuffix), ShouldBeTrue)
			})
		})

		Convey("When RemoveCloneEdgesFromSourceIDs is called with an empty map of IDs", func() {
			err := db.RemoveCloneEdgesFromSourceIDs(ctx, testAttempt, map[string]struct{}{})

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then no query is executed", func() {
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 0)
			})
		})
	})
}

func TestNeptuneDB_GetHierarchyNodeIDs(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		poolMock := &internal.NeptunePoolMock{
			GetStringListFunc: internal.ReturnHierarchyNodeIDs,
		}
		db := mockDB(poolMock)

		Convey("When GetHierarchyNodeIDs is called", func() {
			ids, err := db.GetHierarchyNodeIDs(ctx, testAttempt, testInstanceID, testDimensionName)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected query is sent to Neptune to obtain the cloned hierarchy node IDs, and the expected IDs are returned", func() {
				So(ids, ShouldResemble, testClonedIds)
				expectedQuery := `g.V().hasLabel('_hierarchy_node_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate').id()`
				So(len(poolMock.GetStringListCalls()), ShouldEqual, 1)
				So(poolMock.GetStringListCalls()[0].Query, ShouldResemble, expectedQuery)
			})
		})
	})
}

func TestNeptuneDB_SetNumberOfChildren(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		poolMock := &internal.NeptunePoolMock{
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) (responses []gremgo.Response, err error) {
				return []gremgo.Response{}, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When SetNumberOfChildren is called", func() {
			err := db.SetNumberOfChildren(ctx, testAttempt, testInstanceID, testDimensionName)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected query is sent to Neptune to clone the nodes with the unique provided IDs in any order", func() {
				expectedQuery := `g.V().hasLabel('_hierarchy_node_f0a2f3f2-cc86-4bbb-a549-ffc99c89292c_aggregate').property(single,'numberOfChildren',__.in('hasParent').count())`
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 1)
				So(poolMock.ExecuteCalls()[0].Query, ShouldResemble, expectedQuery)
			})
		})
	})
}

func TestNeptuneDB_SetNumberOfChildrenFromIDs(t *testing.T) {

	Convey("Given a neptune DB", t, func() {
		poolMock := &internal.NeptunePoolMock{
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) (responses []gremgo.Response, err error) {
				return []gremgo.Response{}, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When SetNumberOfChildrenFromIDs is called with a map of IDs", func() {
			err := db.SetNumberOfChildrenFromIDs(ctx, testAttempt, testClonedIds)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected query is sent to Neptune to set the number of children for all provided nodeIDs", func() {
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 1)
				expectedQPrefix := `g.V('`
				expectedQSuffix := `').property(single,'numberOfChildren',__.in('hasParent').count())`
				So(strings.HasPrefix(poolMock.ExecuteCalls()[0].Query, expectedQPrefix), ShouldBeTrue)
				for id := range testClonedIds {
					So(strings.Count(poolMock.ExecuteCalls()[0].Query, id), ShouldEqual, 1)
				}
				So(strings.HasSuffix(poolMock.ExecuteCalls()[0].Query, expectedQSuffix), ShouldBeTrue)
			})
		})

		Convey("When SetNumberOfChildrenFromIDs is called with an empty map of IDs", func() {
			err := db.SetNumberOfChildrenFromIDs(ctx, testAttempt, map[string]struct{}{})

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then no query is executed", func() {
				So(len(poolMock.ExecuteCalls()), ShouldEqual, 0)
			})
		})
	})
}

func TestNeptuneDB_SetHasData(t *testing.T) {

	Convey("Given a neptune DB that returns a code list", t, func() {

		ctx := context.Background()
		attempt := 1
		instanceID := "instanceID"
		dimensionName := "dimensionName"

		poolMock := &internal.NeptunePoolMock{
			GetStringListFunc: internal.ReturnCodesList,
			ExecuteFunc: func(query string, bindings map[string]string, rebindings map[string]string) (responses []gremgo.Response, err error) {
				return []gremgo.Response{}, nil
			},
		}
		db := mockDB(poolMock)

		Convey("When SetHasData is called", func() {
			err := db.SetHasData(ctx, attempt, instanceID, dimensionName)

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the expected query is sent to Neptune to set the hasData property", func() {
				expectedQuery := `g.V().hasLabel('_hierarchy_node_instanceID_dimensionName').as('v').has('code',within(['cpih1dim1S90401','cpih1dim1S90402'])).property(single,'hasData',true)`
				So(poolMock.ExecuteCalls()[0].Query, ShouldEqual, expectedQuery)
			})
		})
	})
}

// mockCodeEdgeMap generates a code-edge map with the expected code and order property for the usedBy edge
func mockNodeIdCodeMap(expectedNodeId, expectedCode string) map[string]json.RawMessage {
	rawNodeId, err := json.Marshal(expectedNodeId)
	So(err, ShouldBeNil)

	rawCode, err := json.Marshal(expectedCode)
	So(err, ShouldBeNil)

	return map[string]json.RawMessage{
		"node_id":   rawNodeId,
		"node_code": rawCode,
	}
}

// mockNodeIdCodeMapResponse generates a nodeId-code map with the expected nodeId and code,
// as returned by Neptune before being processed by graphson into a map (slice representation of the map)
func mockNodeIdCodeMapResponse(expectedNodeId, expectedCode string) json.RawMessage {
	m := mockNodeIdCodeMap(expectedNodeId, expectedCode)
	rawMap, err := SerializeMap(m)
	So(err, ShouldBeNil)
	return rawMap
}
