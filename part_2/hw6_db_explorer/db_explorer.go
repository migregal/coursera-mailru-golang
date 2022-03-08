package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)


type TablesContext struct {
	Tables     map[string]TableInfo
	TableNames []string
}

type TableInfo struct {
	Name   string
	Id     string
	Fields []FieldInfo
	FieldOrdered []string
}

type FieldInfo struct {
	Name     string
	Type     string
	Required bool
	IsKey    bool
}

func (field *FieldInfo) getValueFromString(value string) (interface{}, error) {
	var result interface{}
	var err error
	switch field.Type {
	case "varchar":
		err = nil
		result = value
	case "text":
		err = nil
		result = value
	case "int":
		result, err = strconv.Atoi(value)
	}
	return result, err
}

func (tablesCtxt *TablesContext) containsTable(table string) bool {
	_, ok := tablesCtxt.Tables[table]
	return ok
}

func (table *TableInfo) prepareRow() []interface{} {
	row := make([]interface{}, len(table.Fields))
	for i, field := range table.Fields {
		switch field.Type {
		case "varchar":
			row[i] = new(sql.NullString)
		case "text":
			row[i] = new(sql.NullString)
		case "int":
			row[i] = new(sql.NullInt64)
		}
	}
	return row
}

func (table *TableInfo) prepareInsertQuery() string {
	values := make([]string, len(table.Fields))
	placeholders := make([]string, len(table.Fields))
	for i, field := range table.Fields {
		values[i] = field.Name
		placeholders[i] = "?"
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table.Name, strings.Join(values, ", "),
		strings.Join(placeholders, ", "),
	)
}

func (table *TableInfo) prepareUpdateQuery(params map[string]interface{}) string {
	values := make([]string, 0)
	for k := range params {
		values = append(values, fmt.Sprintf("%s = ?", k))
	}
	return fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = ?",
		table.Name,
		strings.Join(values, ","),
		table.Id,
	)
}

func (table *TableInfo) prepareDeleteQuery() string {
	return fmt.Sprintf(
		"DELETE FROM %s WHERE %s = ?",
		table.Name,
		table.Id,
	)
}

func (table *TableInfo) prepareInsertParameters(params map[string]interface{}) []interface{} {
	result := make([]interface{}, len(table.Fields))
	for i, field := range table.Fields {
		if table.Id == field.Name {
			continue
		}
		if params[field.Name] != nil {
			result[i] = params[field.Name]
			continue
		}
		if !field.Required {
			result[i] = nil
		} else {
			result[i] = field.getDefaultValue()
		}
	}
	return result
}

func (field *FieldInfo) getDefaultValue() interface{} {
	switch field.Type {
	case "varchar":
		return ""
	case "text":
		return ""
	case "int":
		return 0
	}
	return nil
}

func (field *FieldInfo) validateField(value interface{}) error {
	if value == nil && field.Required {
		return fmt.Errorf("field %s have invalid type", field.Name)
	}
	switch value.(type) {
	case float64:
		if field.Type != "int" {
			return fmt.Errorf("field %s have invalid type", field.Name)
		}
	case string:
		if field.Type != "varchar" && field.Type != "text" {
			return fmt.Errorf("field %s have invalid type", field.Name)
		}
	}
	return nil
}

func (table *TableInfo) validateInputParameters(params map[string]interface{}, validateKey bool) error {
	for _, field := range table.Fields {
		value, ok := params[field.Name]
		if !ok {
			continue
		}

		if validateKey && field.IsKey {
			return fmt.Errorf("field %s have invalid type", field.Name)
		}
		if err := field.validateField(value); err != nil {
			return err
		}
	}
	return nil
}

func (table *TableInfo) prepareUpdateParameters(params map[string]interface{}) []interface{} {
	result := make([]interface{}, 0)
	for _, v := range params {
		result = append(result, v)
	}
	return result
}

func (table *TableInfo) transformRow(row []interface{}) map[string]interface{} {
	item := make(map[string]interface{}, len(row))
	for i, v := range row {
		switch v.(type) {
		case *sql.NullString:
			value, ok := v.(*sql.NullString);
			if !ok {
				continue
			}
			if value.Valid {
				item[table.Fields[i].Name] = value.String
			} else {
				item[table.Fields[i].Name] = nil
			}
		case *sql.NullInt64:
			value, ok := v.(*sql.NullInt64)
			if !ok {
				continue
			}
			if value.Valid {
				item[table.Fields[i].Name] = value.Int64
			} else {
				item[table.Fields[i].Name] = nil
			}
		}
	}
	return item
}

func getHandler(db *sql.DB, tablesContext *TablesContext, writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	if path == "/" {
		result, err := json.Marshal(map[string]interface{}{"response": map[string]interface{}{"tables": tablesContext.TableNames}})
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.Write(result)
		return
	}
	fragments := strings.Split(path, "/")
	var err error
	switch len(fragments) {
	case 2:
		tableName := fragments[1]
		if !tablesContext.containsTable(tableName) {
			result, _ := json.Marshal(map[string]interface{}{"error": "unknown table"})
			writer.WriteHeader(http.StatusNotFound)
			writer.Write(result)
			return
		}

		limit := 5
		offset := 0

		if request.URL.Query().Get("limit") != "" {
			t, err := strconv.Atoi(request.URL.Query().Get("limit"))
			if err == nil {
				limit = t
			}
		}
		if request.URL.Query().Get("offset") != "" {
			t, err := strconv.Atoi(request.URL.Query().Get("offset"))
			if err == nil {
				offset = t
			}
		}

		rows, err := getRows(db, tablesContext.Tables[tableName], limit, offset)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			println(err.Error())
			return
		}
		result, err := json.Marshal(
			map[string]interface{}{"response": map[string]interface{}{"records": rows}})
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			println(err.Error())
			return
		}
		writer.Write(result)
	case 3:
		table := fragments[1]
		id := fragments[2]
		if !tablesContext.containsTable(table) {
			writer.WriteHeader(http.StatusNotFound)
			println(err.Error())
			return
		}
		rows, err := getRow(db, tablesContext.Tables[table], id)
		if err != nil {
			writer.WriteHeader(http.StatusNotFound)
			result, _ := json.Marshal(map[string]string{"error": "record not found"})
			writer.Write(result)
			return
		}
		result, err := json.Marshal(
			map[string]interface{}{"response": map[string]interface{}{"record": rows}})
		writer.Write(result)

	}
}

func deleteHandler(db *sql.DB, tablesContext *TablesContext, writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	fragments := strings.Split(path, "/")
	tableName := fragments[1]
	id := fragments[2]
	if !tablesContext.containsTable(tableName) {
		result, _ := json.Marshal(map[string]interface{}{"error": "unknown table"})
		writer.WriteHeader(http.StatusNotFound)
		writer.Write(result)
		return
	}
	table := tablesContext.Tables[tableName]
	result, err := deleteRow(db, table, id)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		println(err.Error())
		return
	}
	resultBytes, _ := json.Marshal(map[string]interface{}{"response": map[string]interface{}{"deleted": result}})
	writer.Write(resultBytes)
}

func postHandler(db *sql.DB, tablesContext *TablesContext, writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	fragments := strings.Split(path, "/")
	tableName := fragments[1]
	id := fragments[2]
	if !tablesContext.containsTable(tableName) {
		result, _ := json.Marshal(map[string]interface{}{"error": "unknown table"})
		writer.WriteHeader(http.StatusNotFound)
		writer.Write(result)
		return
	}
	table := tablesContext.Tables[tableName]
	decoder := json.NewDecoder(request.Body)
	requestParams := make(map[string]interface{}, len(table.Fields))
	decoder.Decode(&requestParams)
	validationError := table.validateInputParameters(requestParams, true)
	if validationError != nil {
		result, _ := json.Marshal(map[string]interface{}{"error": validationError.Error()})
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write(result)
		return
	}
	fmt.Printf("Got parameters %#v\n", requestParams)
	table = tablesContext.Tables[tableName]
	result, err := updateRow(db, table, id, requestParams)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		println(err.Error())
		return
	}
	resultBytes, _ := json.Marshal(map[string]interface{}{"response": map[string]interface{}{"updated": result}})
	writer.Write(resultBytes)
}

func putHandler(db *sql.DB, tablesContext *TablesContext, writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	fragments := strings.Split(path, "/")
	tableName := fragments[1]
	if !tablesContext.containsTable(tableName) {
		result, _ := json.Marshal(map[string]interface{}{"error": "unknown table"})
		writer.WriteHeader(http.StatusNotFound)
		writer.Write(result)
		return
	}
	table := tablesContext.Tables[tableName]

	decoder := json.NewDecoder(request.Body)
	requestParams := make(map[string]interface{}, len(table.Fields))
	decoder.Decode(&requestParams)
	fmt.Printf("Got parameters %#v\n", requestParams)
	result, err := insertRow(db, tablesContext.Tables[tableName], requestParams)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		println(err.Error())
		return
	}
	resultBytes, _ := json.Marshal(map[string]interface{}{"response": map[string]interface{}{table.Id: result}})
	writer.Write(resultBytes)
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	tablesContext, err := initContext(db)
	serverMux := http.NewServeMux()
	if err != nil {
		panic(err)
	}
	serverMux.HandleFunc("/", func (writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
		case http.MethodGet:
			getHandler(db, tablesContext, writer, request)
		case http.MethodDelete:
			deleteHandler(db, tablesContext, writer, request)
		case http.MethodPost:
			postHandler(db, tablesContext, writer, request)
		case http.MethodPut:
			putHandler(db, tablesContext, writer, request)
		}
	})
	return serverMux, nil
}

func (table *TableInfo) extractParams(values url.Values) map[string]interface{} {
	result := make(map[string]interface{})
	for _, field := range table.Fields {
		if len(values[field.Name]) == 0 {

			result[field.Name] = nil
		} else {
			v, _ := field.getValueFromString(values[field.Name][0])
			result[field.Name] = v
		}
	}
	return result
}

func getRow(db *sql.DB, table TableInfo, id interface{}) (map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", strings.Join(table.FieldOrdered, ","), table.Name, table.Id)
	data := table.prepareRow()
	row := db.QueryRow(query, id)
	err := row.Scan(data...)
	if err != nil {
		return nil, err
	}
	return table.transformRow(data), nil
}

func getTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var tableName string
		rows.Scan(&tableName)
		result = append(result, tableName)

	}
	return result, nil
}

func insertRow(db *sql.DB, table TableInfo, params map[string]interface{}) (int64, error) {
	query := table.prepareInsertQuery()
	queryParams := table.prepareInsertParameters(params)
	fmt.Printf("parameters %v\n", queryParams)
	res, err := db.Exec(query, queryParams...)
	if err != nil {
		return 0, err
	} else {
		result, _ := res.LastInsertId()
		return result, nil
	}
}

func updateRow(db *sql.DB, table TableInfo, id interface{}, params map[string]interface{}) (int64, error) {
	query := table.prepareUpdateQuery(params)
	queryParams := table.prepareUpdateParameters(params)
	queryParams = append(queryParams, id)
	res, err := db.Exec(query, queryParams...)
	if err != nil {
		return 0, err
	} else {
		result, _ := res.RowsAffected()
		return result, nil
	}
}

func deleteRow(db *sql.DB, table TableInfo, id interface{}) (int64, error) {
	query := table.prepareDeleteQuery()
	res, err := db.Exec(query, id)
	if err != nil {
		return 0, err
	} else {
		result, _ := res.RowsAffected()
		return result, nil
	}
}

func initContext(db *sql.DB) (*TablesContext, error) {
	tables, err := getTables(db)
	if err != nil {
		return nil, err
	}
	result := new(TablesContext)
	result.TableNames = tables
	result.Tables = make(map[string]TableInfo, len(tables))
	for _, table := range tables {
		rows, err := db.Query("SELECT column_name, IF (column_key='PRI', true, false) AS 'key', DATA_TYPE, IF (is_nullable='NO', true, false) AS nullable FROM information_schema.columns WHERE table_name = ? AND table_schema=database()", table)
		if err != nil {
			return nil, err
		}
		var keyName string
		fields := make([]FieldInfo, 0)
		for rows.Next() {
			f := new(FieldInfo)
			rows.Scan(&f.Name, &f.IsKey, &f.Type, &f.Required)
			if f.IsKey {
				keyName = f.Name
			}
			fields = append(fields, *f)
		}
		var fieldNames []string
		for _, v := range fields {
			fieldNames = append(fieldNames, v.Name)
		}
		result.Tables[table] = TableInfo{
			Name:   table,
			Id:     keyName,
			Fields: fields,
			FieldOrdered: fieldNames,
		}
		rows.Close()
	}
	return result, nil
}

func getRows(db *sql.DB, table TableInfo, limit int, offset int) ([]interface{}, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT %s FROM %s LIMIT %d OFFSET %d", strings.Join(table.FieldOrdered, ","), table.Name, limit, offset))
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	result := []interface{}{}
	for rows.Next() {
		row := table.prepareRow()
		rows.Scan(row...)
		result = append(result, table.transformRow(row))
	}

	return result, nil
}
