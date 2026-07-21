package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/app"
	"github.com/Jcorrieri/uf-marketplace/backend/config"
	"github.com/Jcorrieri/uf-marketplace/backend/database"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
	"github.com/gin-gonic/gin"
)

func TestNewRouterUsesInjectedAuthenticationConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := database.OpenSQLite("file:router_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := database.Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	user, err := services.NewUserService(db).Create(
		context.Background(),
		services.CreateUserRequest{
			Email:     "router-test@ufl.edu",
			Password:  "password",
			FirstName: "Router",
			LastName:  "Test",
		},
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	const jwtSecret = "router-test-secret"
	const cookieName = "router_test_session"
	token, err := utils.GenerateToken(user.ID, jwtSecret)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	router := app.NewRouter(db, config.Config{
		ServerAddress:     "localhost:8080",
		DatabasePath:      "unused.db",
		JWTSecret:         jwtSecret,
		SessionCookieName: cookieName,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	request.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body)
	}
}
