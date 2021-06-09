package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gjg-sarismet/db"
	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
)

func TestGetLeaderBoard(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.GetLeaderBoard(c)) {
		var responseUsers []db.User
		json.Unmarshal(rec.Body.Bytes(), &responseUsers)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseUsers[1].Display_Name, users[0].Display_Name)
	}
}
