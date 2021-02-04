package pxapi

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/stretchr/testify/assert"
	"pixielabs.ai/pixielabs/src/api/go/pxapi/errdefs"
	"pixielabs.ai/pixielabs/src/api/go/pxapi/types"
	vizierapipb "pixielabs.ai/pixielabs/src/api/public/vizierapipb"
	vizierpb "pixielabs.ai/pixielabs/src/api/public/vizierapipb"
)

func makeErrorResponse(message string) *vizierpb.ExecuteScriptResponse {
	return &vizierpb.ExecuteScriptResponse{
		Status: &vizierpb.Status{
			Code:    int32(codes.InvalidArgument),
			Message: message,
		},
	}
}

func makeCompilerError(line uint64, col uint64, message string) *vizierpb.ExecuteScriptResponse {
	errResp := makeErrorResponse(message)
	errResp.Status.ErrorDetails = []*vizierpb.ErrorDetails{
		{
			Error: &vizierpb.ErrorDetails_CompilerError{
				CompilerError: &vizierpb.CompilerError{
					Line:    line,
					Column:  col,
					Message: message,
				},
			},
		},
	}
	return errResp
}

type FakeTable struct {
	name     string
	id       string
	relation *vizierpb.Relation
}

func NewFakeTable(name string, id string, relation *vizierpb.Relation) *FakeTable {
	return &FakeTable{
		name:     name,
		id:       id,
		relation: relation,
	}
}

func okStatus() *vizierpb.Status {
	return &vizierpb.Status{Code: 0}
}

func makeInt64Column(data []int64) *vizierpb.Column {
	return &vizierpb.Column{
		ColData: &vizierpb.Column_Int64Data{
			Int64Data: &vizierpb.Int64Column{
				Data: data,
			},
		},
	}
}

func makeStringColumn(data []string) *vizierpb.Column {
	return &vizierpb.Column{
		ColData: &vizierpb.Column_StringData{
			StringData: &vizierpb.StringColumn{
				Data: data,
			},
		},
	}
}

func (ft *FakeTable) MetadataResponse() *vizierpb.ExecuteScriptResponse {
	return &vizierpb.ExecuteScriptResponse{
		Status: okStatus(),
		Result: &vizierpb.ExecuteScriptResponse_MetaData{
			MetaData: &vizierpb.QueryMetadata{
				Name:     ft.name,
				Relation: ft.relation,
				ID:       ft.id,
			},
		},
	}
}

func (ft *FakeTable) rowBatch(cols []*vizierpb.Column, numRows int64, eow, eos bool) *vizierpb.RowBatchData {
	return &vizierpb.RowBatchData{
		TableID: ft.id,
		Cols:    cols,
		Eow:     eow,
		Eos:     eos,
		NumRows: numRows,
	}
}

func (ft *FakeTable) RowBatchResponse(cols []*vizierpb.Column, numRows int64) *vizierpb.ExecuteScriptResponse {
	return &vizierpb.ExecuteScriptResponse{
		Status: okStatus(),
		Result: &vizierpb.ExecuteScriptResponse_Data{
			Data: &vizierpb.QueryData{
				Batch: ft.rowBatch(cols, numRows, false, false),
			},
		},
	}
}

func (ft *FakeTable) EndResponse() *vizierpb.ExecuteScriptResponse {
	return &vizierpb.ExecuteScriptResponse{
		Status: okStatus(),
		Result: &vizierpb.ExecuteScriptResponse_Data{
			Data: &vizierpb.QueryData{
				Batch: ft.rowBatch([]*vizierpb.Column{}, 0, true, true),
			},
		},
	}
}

func noSemTypeColInfo(name string, dataType vizierpb.DataType) *vizierpb.Relation_ColumnInfo {
	return &vizierpb.Relation_ColumnInfo{
		ColumnName:         name,
		ColumnType:         dataType,
		ColumnSemanticType: vizierpb.ST_NONE,
	}
}

type singleInt64Handler struct {
	ColumnName string
	Data       []int64
}

func (t *singleInt64Handler) HandleInit(ctx context.Context, metadata types.TableMetadata) error {
	if len(metadata.ColInfo) != 1 {
		return errors.New("handler only accepts a single col")
	}
	if metadata.ColInfo[0].Type != vizierapipb.INT64 {
		return errors.New("handler only int columns")
	}
	t.ColumnName = metadata.ColInfo[0].Name
	return nil
}

func (t *singleInt64Handler) HandleRecord(ctx context.Context, r *types.Record) error {
	t.Data = append(t.Data, r.Data[0].(*types.Int64Value).Value())
	return nil
}

func (t *singleInt64Handler) HandleDone(ctx context.Context) error {
	return nil
}

type int64TableMux struct {
	Tables map[string]*singleInt64Handler
}

func (s *int64TableMux) AcceptTable(ctx context.Context, metadata types.TableMetadata) (TableRecordHandler, error) {
	s.Tables[metadata.Name] = &singleInt64Handler{}
	return s.Tables[metadata.Name], nil
}

func newTableMux() *int64TableMux {
	return &int64TableMux{
		Tables: make(map[string]*singleInt64Handler),
	}
}

func TestProcessOneTable(t *testing.T) {
	results := newScriptResults()
	tm := newTableMux()
	results.tm = tm

	relation := &vizierpb.Relation{
		Columns: []*vizierpb.Relation_ColumnInfo{
			noSemTypeColInfo("http_status", vizierpb.INT64),
		},
	}

	table := NewFakeTable("http_table", "abc", relation)

	messages := []*vizierpb.ExecuteScriptResponse{
		table.MetadataResponse(),
		table.RowBatchResponse([]*vizierpb.Column{
			makeInt64Column([]int64{1, 2}),
		}, 2),
		table.RowBatchResponse([]*vizierpb.Column{
			makeInt64Column([]int64{3, 4, 5}),
		}, 3),
		table.EndResponse(),
	}

	ctx := context.Background()
	for _, msg := range messages {
		assert.Nil(t, results.handleGRPCMsg(ctx, msg))
	}

	assert.Equal(t, len(tm.Tables), 1)
	httpTable, ok := tm.Tables["http_table"]
	if !ok {
		t.Fatalf("'%s' not found in data", "http_table")
	}

	assert.Equal(t, "http_status", httpTable.ColumnName)
	assert.Equal(t, []int64{1, 2, 3, 4, 5}, httpTable.Data)
}

func DISABLED_TestProcessNoEnd(t *testing.T) {
	results := newScriptResults()
	tm := newTableMux()
	results.tm = tm

	relation := &vizierpb.Relation{
		Columns: []*vizierpb.Relation_ColumnInfo{
			noSemTypeColInfo("http_status", vizierpb.INT64),
		},
	}

	table := NewFakeTable("http_table", "abc", relation)

	messages := []*vizierpb.ExecuteScriptResponse{
		table.MetadataResponse(),
		table.RowBatchResponse([]*vizierpb.Column{
			makeInt64Column([]int64{1, 2}),
		}, 2),
		// Don't send an EndResponse
	}

	ctx := context.Background()
	for _, msg := range messages {
		assert.Nil(t, results.handleGRPCMsg(ctx, msg))
	}

	// TODO(zasar) Should we error out if we don't receive an eos?
	err := results.Close()
	assert.Error(t, err)
}

func TestReceiveDataAfterEOS(t *testing.T) {
	results := newScriptResults()
	tm := newTableMux()
	results.tm = tm

	relation := &vizierpb.Relation{
		Columns: []*vizierpb.Relation_ColumnInfo{
			noSemTypeColInfo("http_status", vizierpb.INT64),
		},
	}

	table := NewFakeTable("http_table", "abc", relation)

	ctx := context.Background()
	assert.Nil(t, results.handleGRPCMsg(ctx, table.MetadataResponse()))
	assert.Nil(t, results.handleGRPCMsg(ctx, table.EndResponse()))
	err := results.handleGRPCMsg(ctx, table.RowBatchResponse([]*vizierpb.Column{
		makeInt64Column([]int64{1, 2}),
	}, 2))

	assert.Equal(t, errdefs.ErrInternalDataAfterEOS, err)
}

func TestProcessWrongColumn(t *testing.T) {
	results := newScriptResults()
	tm := newTableMux()
	results.tm = tm

	relation := &vizierpb.Relation{
		Columns: []*vizierpb.Relation_ColumnInfo{
			noSemTypeColInfo("http_status", vizierpb.INT64),
		},
	}

	table := NewFakeTable("http_table", "abc", relation)

	ctx := context.Background()
	assert.Nil(t, results.handleGRPCMsg(ctx, table.MetadataResponse()))
	// Sends over a string column where an int column should be.
	err := results.handleGRPCMsg(ctx, table.RowBatchResponse([]*vizierpb.Column{
		makeStringColumn([]string{"a", "b"}),
	}, 2))

	assert.Equal(t, err, errdefs.ErrInternalMismatchedType)
}

func TestDuplicateMetadata(t *testing.T) {
	results := newScriptResults()
	tm := newTableMux()
	results.tm = tm

	relation := &vizierpb.Relation{
		Columns: []*vizierpb.Relation_ColumnInfo{
			noSemTypeColInfo("http_status", vizierpb.INT64),
		},
	}

	table := NewFakeTable("http_table", "abc", relation)

	ctx := context.Background()
	assert.Nil(t, results.handleGRPCMsg(ctx, table.MetadataResponse()))
	// SEnd the same message over twice.
	err := results.handleGRPCMsg(ctx, table.MetadataResponse())
	assert.Equal(t, err, errdefs.ErrInternalDuplicateTableMetadata)
}

func TestNonExistantTable(t *testing.T) {
	results := newScriptResults()
	tm := newTableMux()
	results.tm = tm

	relation := &vizierpb.Relation{
		Columns: []*vizierpb.Relation_ColumnInfo{
			noSemTypeColInfo("http_status", vizierpb.INT64),
		},
	}

	table := NewFakeTable("http_table", "abc", relation)

	ctx := context.Background()
	// Send data from a table that has not sent a metadata response.
	err := results.handleGRPCMsg(ctx, table.RowBatchResponse([]*vizierpb.Column{
		makeInt64Column([]int64{1, 2}),
	}, 2))
	assert.Equal(t, err, errdefs.ErrInternalMissingTableMetadata)
}

func TestProcessTwoTables(t *testing.T) {
	tableID1 := "abc"
	tableID2 := "def"

	relation := &vizierpb.Relation{
		Columns: []*vizierpb.Relation_ColumnInfo{
			noSemTypeColInfo("http_status", vizierpb.INT64),
		},
	}
	table1 := NewFakeTable("table1", tableID1, relation)
	table2 := NewFakeTable("table2", tableID2, relation)

	messages := []*vizierpb.ExecuteScriptResponse{
		table1.MetadataResponse(),
		table1.RowBatchResponse([]*vizierpb.Column{
			makeInt64Column([]int64{1, 2}),
		}, 2),
		table1.RowBatchResponse([]*vizierpb.Column{
			makeInt64Column([]int64{3, 4, 5}),
		}, 3),
		table1.EndResponse(),

		table2.MetadataResponse(),
		table2.RowBatchResponse([]*vizierpb.Column{
			makeInt64Column([]int64{7, 8, 9, 10}),
		}, 4),
		table2.EndResponse(),
	}

	tm := newTableMux()
	results := newScriptResults()
	results.tm = tm

	ctx := context.Background()
	for _, msg := range messages {
		assert.Nil(t, results.handleGRPCMsg(ctx, msg))
	}

	assert.Equal(t, len(tm.Tables), 2)
	table1Data, ok := tm.Tables["table1"]
	if !ok {
		t.Fatalf("table not found")
	}

	table2Data, ok := tm.Tables["table2"]
	if !ok {
		t.Fatalf("table not found")
	}

	assert.Equal(t, "http_status", table1Data.ColumnName)
	assert.Equal(t, "http_status", table2Data.ColumnName)

	assert.Equal(t, []int64{1, 2, 3, 4, 5}, table1Data.Data)
	assert.Equal(t, []int64{7, 8, 9, 10}, table2Data.Data)
}

func DISABLED_TestExecuteScriptGetsScriptError(t *testing.T) {
	// TODO(zasgar) propagate the script error to the user.

	results := newScriptResults()
	results.tm = newTableMux()

	ctx := context.Background()
	err := results.handleGRPCMsg(ctx, makeCompilerError(1, 2, "name 'aa' is not defined"))
	assert.NotNil(t, err)
	assert.Regexp(t, "name.*is not defined", err)
}

func DISABLED_TestExecuteScriptGetsOtherErrorOnStream(t *testing.T) {
	// TODO(zasgar) propagate the script error to the user.
	results := newScriptResults()
	results.tm = newTableMux()

	ctx := context.Background()
	err := results.handleGRPCMsg(ctx, makeErrorResponse("Script should not be empty."))
	assert.NotNil(t, err)
	assert.Regexp(t, "Script should not be empty.", err)
}
