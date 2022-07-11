package main

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestUsersRoute(t *testing.T) {
	tests := []struct {
		description  string
		route        string // route path to test
		expectedCode int    // expected HTTP status code
		expectedBody string // expected response body
	}{
		{
			description:  "get HTTP status 401 because of missing authorization header",
			route:        "/",
			expectedCode: 401,
			expectedBody: "Unauthorized",
		},
		{
			description:  "get HTTP status 404, when route is not exists",
			route:        "/notfound",
			expectedCode: 404,
			expectedBody: "Cannot GET /notfound",
		},
		{
			description:  "get HTTP status 200, when user does not exist",
			route:        "/users/62cb8c67311878d3a15f1388",
			expectedCode: 500,
			expectedBody: "Error finding user",
		},
		{
			description:  "get HTTP status 200, when user exists",
			route:        "/users/62cb8c67311878d3a15f1389",
			expectedCode: 200,
			expectedBody: `{"ID":"62cb8c67311878d3a15f1389","first_name":"ken","last_name":"lam","gender":"male","age":2}`,
		},
	}

	app := fiber.New()

	SetupRoutes(app)

	for _, test := range tests {
		req := httptest.NewRequest("GET", test.route, nil)
		resp, _ := app.Test(req, 1)

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}

		// assert HTTP status code
		assert.Equal(t, test.expectedCode, resp.StatusCode, test.description)
		// assert response body
		assert.Equal(t, test.expectedBody, string(body), test.description)
	}
}
