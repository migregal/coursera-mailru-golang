package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

/*
	Data structs
*/

type XmlUser struct {
	Id         int    `xml:"id" json:"id"`
	Name       string `xml:"-" json:"-"`
	FirstName  string `xml:"first_name" json:"-"`
	SecondName string `xml:"last_name" json:"-"`
	Age        int    `xml:"age" json:"age"`
	About      string `xml:"about" json:"about"`
	Gender     string `xml:"gender" json:"gender"`
}

type XmlUsers struct {
	XMLName xml.Name  `xml:"root"`
	Users   []XmlUser `xml:"row"`
}

func (u *XmlUser) compareById(other *XmlUser) int {
	if u.Id > other.Id {
		return OrderByDesc
	}

	if u.Id < other.Id {
		return OrderByAsc
	}

	return OrderByAsIs
}

func (u *XmlUser) compareByAge(other *XmlUser) int {
	if u.Age > other.Age {
		return OrderByDesc
	}

	if u.Age < other.Age {
		return OrderByAsc
	}

	return OrderByAsIs
}

func (u *XmlUser) compareByName(other *XmlUser) int {
	name := u.getFullName()
	name2 := other.getFullName()
	if name > name2 {
		return OrderByDesc
	}

	if name < name2 {
		return OrderByAsc
	}

	return OrderByAsIs
}

func (u *XmlUser) getFullName() string {
	return u.FirstName + " " + u.SecondName
}

func (u *XmlUser) MarshalJSON() ([]byte, error) {
	type Copy XmlUser

	return json.Marshal(&struct {
		Name string `json:"name"`
		*Copy
	}{
		Name: u.getFullName(),
		Copy: (*Copy)(u),
	})
}

/*
	SearchServer part
*/

type SearchServer struct {
	pathToFile string
}

func (ss *SearchServer) getUsers(params *SearchRequest) ([]XmlUser, error) {
	rawUsers, err := getUsersFromFile(ss.pathToFile)
	if err != nil {
		return nil, err
	}

	var resultUsers []XmlUser

	if params.Query == "" {
		resultUsers = rawUsers
	} else {
		for _, user := range rawUsers {
			nameContainsQuery := strings.Contains(user.getFullName(), params.Query)
			aboutContainsQuery := strings.Contains(user.About, params.Query)

			if nameContainsQuery || aboutContainsQuery {
				resultUsers = append(resultUsers, user)
			}
		}
	}

	if params.OrderBy != 0 && params.OrderField != "" {
		sortUsers(resultUsers, params.OrderField, params.OrderBy)
	}

	if params.Offset+params.Limit > len(resultUsers) {
		return resultUsers[params.Offset:], nil
	}

	return resultUsers[params.Offset:params.Limit], nil
}

func getUsersFromFile(pathToFile string) ([]XmlUser, error) {
	file, err := os.Open(pathToFile)
	if err != nil {
		return nil, errors.New("invalid resource path")
	}
	defer func() { _ = file.Close() }()

	var usersList XmlUsers
	if err := xml.NewDecoder(file).Decode(&usersList); err != nil {
		return nil, errors.New("file decoding error")
	}

	return usersList.Users, nil
}

func sortUsers(users []XmlUser, orderField string, orderBy int) {
	sort.Slice(users, func(i, j int) bool {
		if orderField == "Age" {
			return users[i].compareByAge(&users[j]) == orderBy
		}

		if orderField == "Name" {
			return users[i].compareByName(&users[j]) == orderBy
		}

		return users[i].compareById(&users[j]) == orderBy
	})
}

func SearchServerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	token := r.Header.Get("AccessToken")
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if token != testToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	searchRequest, err := getValidatedInput(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		if err.Error() == ErrorBadOrderField {
			_, _ = io.WriteString(w, fmt.Sprintf(`{"StatusCode": 400, "Error": "`+ErrorBadOrderFieldMsg+`"}`))
		} else {
			_, _ = io.WriteString(w, fmt.Sprintf(`{"StatusCode": 400, "OrderField": "%s"}`, err.Error()))
		}

		return
	}

	searchServer := SearchServer{"./dataset.xml"}

	users, err := searchServer.getUsers(searchRequest)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, fmt.Sprintf(`{"StatusCode": 500, "error": "%s"}`, err.Error()))
		return
	}

	usersJson, err := json.Marshal(users)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, fmt.Sprintf(`{"StatusCode": 500, "error": "Invalid data for json encoding"}`))
		return
	}

	_, _ = io.WriteString(w, string(usersJson))
}

func getValidatedInput(r *http.Request) (*SearchRequest, error) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))

	if err != nil {
		return nil, errors.New("limit")
	}

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))

	if err != nil {
		return nil, errors.New("offset")
	}

	orderBy, err := strconv.Atoi(r.URL.Query().Get("order_by"))

	if err != nil {
		return nil, errors.New("order_by")

	}

	orderField := r.URL.Query().Get("order_field")
	if orderField == "" {
		return nil, errors.New(ErrorBadOrderFieldMsg)
	}

	query := r.URL.Query().Get("query")

	return &SearchRequest{
		limit, offset, query, orderField, orderBy,
	}, nil
}

/*
	Tests part
*/

const testToken string = "8bee2936-5366-4b1b-bfbf-92ef627fa4b0"

// Response errors
const (
	ErrorBadOrderFieldMsg    = `ErrorBadOrderField`
	WrongLimitMsg            = "limit must be > 0"
	WrongOffsetMsg           = "offset must be > 0"
	BadAccessToken           = "Bad AccessToken"
	FatalError               = "SearchServer fatal error"
	InvalidOrderField        = "OrderFeld %s invalid"
	JSONUnpackingError       = "cant unpack error json"
	JSONUnpackingResultError = "cant unpack result json"
	UnknownBadRequestError   = "unknown bad request error"
)

// Tests error msgs
const (
	InvalidErrorMsg  = "Invalid error text"
	ErrorIsNilFor    = "Error is nil for %s"
	ErrorIsNotNilFor = "Error is not nil for %s"
)

func TestRequestLimitBelowZero(t *testing.T) {
	var searchClient SearchClient

	_, err := searchClient.FindUsers(SearchRequest{Limit: -5})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, "Limit < 0"))
	} else if err.Error() != WrongLimitMsg {
		t.Error(InvalidErrorMsg)
	}
}

func TestRequestOffsetBelowZero(t *testing.T) {
	var searchClient SearchClient

	_, err := searchClient.FindUsers(SearchRequest{Offset: -5})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, "Offset < 0"))
	} else if err.Error() != WrongOffsetMsg {
		t.Error(InvalidErrorMsg)
	}
}

func TestNoToken(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(SearchServerHandler))
	defer searchService.Close()
	searchClient := &SearchClient{"Wrong token", searchService.URL}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, "invalid token"))
	} else if err.Error() != BadAccessToken {
		t.Error(InvalidErrorMsg)
	}
}

func TestLongServerResponse(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		return
	}))

	defer searchService.Close()
	searchClient := &SearchClient{testToken, searchService.URL}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error("Timeout reached but no error")
	} else if !strings.Contains(err.Error(), "timeout") {
		t.Error(InvalidErrorMsg)
	}
}

func TestEmptyUrl(t *testing.T) {
	searchClient := &SearchClient{testToken, ""}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, "nil url"))
	} else if !strings.Contains(err.Error(), "unknown error") {
		t.Error(InvalidErrorMsg)
	}
}

func TestServer500Code(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}))

	defer searchService.Close()
	searchClient := &SearchClient{testToken, searchService.URL}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, FatalError))
	} else if err.Error() != FatalError {
		t.Error(InvalidErrorMsg)
	}
}

func TestOrderFieldValidationErrors(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, fmt.Sprintf(`{"StatusCode": 400, "Error": "`+ErrorBadOrderFieldMsg+`"}`))
		return
	}))

	defer searchService.Close()

	searchClient := &SearchClient{testToken, searchService.URL}

	_, err := searchClient.FindUsers(SearchRequest{OrderField: "smth"})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, fmt.Sprintf(InvalidOrderField, "smth")))
	} else if err.Error() != fmt.Sprintf(InvalidOrderField, "smth") {
		t.Error(InvalidErrorMsg)
	}
}

func TestOrderFieldValidationWrongJson(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, fmt.Sprintf(`Error`))
		return
	}))

	defer searchService.Close()

	searchClient := &SearchClient{testToken, searchService.URL}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, JSONUnpackingError))
	} else if !strings.Contains(err.Error(), JSONUnpackingError) {
		t.Error(InvalidErrorMsg)
	}
}

func TestValidationErrors(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, fmt.Sprintf(`{"StatusCode": 400, "OrderField": "Limit"}`))
		return
	}))

	defer searchService.Close()

	searchClient := &SearchClient{testToken, searchService.URL}

	_, err := searchClient.FindUsers(SearchRequest{OrderField: "smth"})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, UnknownBadRequestError))
	} else if !strings.Contains(err.Error(), UnknownBadRequestError) {
		t.Error(InvalidErrorMsg)
	}
}

func TestCorrectRequest(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(SearchServerHandler))
	defer searchService.Close()
	searchClient := &SearchClient{testToken, searchService.URL}

	result, err := searchClient.FindUsers(
		SearchRequest{
			Limit:      1,
			Offset:     0,
			OrderField: "Id",
			OrderBy:    OrderByAsc,
		})

	if err != nil {
		t.Error(fmt.Sprintf(ErrorIsNotNilFor, "Correct request"))
	} else if !result.NextPage {
		t.Error("NextPage is not valid")
	} else if len(result.Users) != 1 {
		t.Error("Wrong users amount")
	}
}

func TestCorrectMaximumLimit(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(SearchServerHandler))
	defer searchService.Close()
	searchClient := &SearchClient{testToken, searchService.URL}

	result, err := searchClient.FindUsers(
		SearchRequest{
			Limit:      50,
			Offset:     0,
			OrderField: "Id",
			OrderBy:    OrderByAsc,
		})

	if err != nil {
		t.Error(fmt.Sprintf(ErrorIsNotNilFor, "Correct maximum limit"))
		return
	} else if len(result.Users) != 25 {
		t.Error("Wrong users amount")
	}
}

func TestQuery(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(SearchServerHandler))
	defer searchService.Close()
	searchClient := &SearchClient{testToken, searchService.URL}
	query := "consequat elit ipsum"

	result, err := searchClient.FindUsers(
		SearchRequest{
			Limit:      500,
			Offset:     0,
			OrderField: "Id",
			OrderBy:    OrderByAsc,
			Query:      query,
		})

	if err != nil {
		t.Error(fmt.Sprintf(ErrorIsNotNilFor, "Query work result"))
		return
	}

	for _, user := range result.Users {
		nameContainsQuery := strings.Contains(user.Name, query)
		aboutContainsQuery := strings.Contains(user.About, query)

		if !(nameContainsQuery || aboutContainsQuery) {
			t.Error("Wrong result")
			break
		}
	}
}

func TestInvalidJsonError(t *testing.T) {
	searchService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "hello world")
		return
	}))

	defer searchService.Close()

	searchClient := &SearchClient{testToken, searchService.URL}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error(fmt.Sprintf(ErrorIsNilFor, JSONUnpackingResultError))
	} else if !strings.Contains(err.Error(), JSONUnpackingResultError) {
		t.Error(InvalidErrorMsg)
	}
}
