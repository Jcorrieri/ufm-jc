package app

import (
	"github.com/Jcorrieri/uf-marketplace/backend/config"
	"github.com/Jcorrieri/uf-marketplace/backend/handlers"
	"github.com/Jcorrieri/uf-marketplace/backend/middleware"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterAuthRoutes(public *gin.RouterGroup, authHandler *handlers.AuthHandler) {
	public.POST("/register", authHandler.Register)
	public.POST("/login", authHandler.Login)
	public.POST("/logout", authHandler.Logout)
}

func RegisterPasswordResetRoutes(
	public *gin.RouterGroup,
	passwordResetHandler *handlers.PasswordResetHandler,
) {
	public.POST("/forgot-password", passwordResetHandler.ForgotPassword)
	public.POST("/reset-password", passwordResetHandler.ResetPassword)
}

func RegisterUserRoutes(protected *gin.RouterGroup, userHandler *handlers.UserHandler) {
	protected.PUT("/users/me/profile-image", userHandler.UploadProfileImage)
	protected.GET("/users/:id", userHandler.GetUserById)
	protected.GET("/users/me", userHandler.GetCurrentUser)
	protected.DELETE("/users/me", userHandler.DeleteUser)
	protected.PUT("/users/me", userHandler.UpdateSettings)
}

func RegisterListingsRoutes(
	public *gin.RouterGroup,
	protected *gin.RouterGroup,
	listingHandler *handlers.ListingHandler,
) {
	public.GET("/listings", listingHandler.GetListings)
	protected.GET("/listings/me", listingHandler.GetMyListings)
	protected.POST("/listings", listingHandler.CreateListing)
	protected.PUT("/listings/:id", listingHandler.UpdateListing)
	protected.DELETE("/listings/:id", listingHandler.DeleteListing)
}

func RegisterImageRoutes(public *gin.RouterGroup, imageHandler *handlers.ImageHandler) {
	public.GET("/images/:imageId", imageHandler.GetImage)
}

func RegisterOrderRoutes(protected *gin.RouterGroup, orderHandler *handlers.OrderHandler) {
	protected.POST("/orders", orderHandler.CreateOrder)
	protected.GET("/orders/me", orderHandler.GetMyOrders)
}

func RegisterChatRoutes(protected *gin.RouterGroup, chatHandler *handlers.ChatHandler) {
	protected.POST("/conversations", chatHandler.StartConversation)
	protected.GET("/conversations", chatHandler.GetConversations)
	protected.GET("/conversations/:id/messages", chatHandler.GetMessages)
	protected.GET("/ws/chat/:id", chatHandler.ServeWs)
}

// NewRouter constructs the HTTP application from explicit dependencies.
func NewRouter(db *gorm.DB, configuration config.Config) *gin.Engine {
	authService := services.NewAuthService(db, configuration.JWTSecret)
	passwordResetService := services.NewPasswordResetService(db)
	userService := services.NewUserService(db)
	listingService := services.NewListingService(db)
	imageService := services.NewImageService(db)
	orderService := services.NewOrderService(db)
	chatService := services.NewChatService(db)

	authHandler := handlers.NewAuthHandler(
		authService,
		userService,
		configuration.SessionCookieName,
	)
	passwordResetHandler := handlers.NewPasswordResetHandler(passwordResetService)
	userHandler := handlers.NewUserHandler(userService)
	listingHandler := handlers.NewListingHandler(listingService)
	imageHandler := handlers.NewImageHandler(imageService)
	orderHandler := handlers.NewOrderHandler(orderService, listingService)
	hub := services.NewHub()
	go hub.Run()
	chatHandler := handlers.NewChatHandler(chatService, hub)

	authMiddleware := middleware.AuthMiddleware(
		configuration.JWTSecret,
		configuration.SessionCookieName,
	)

	router := gin.Default()
	api := router.Group("/api")
	auth := api.Group("/auth")
	protected := api.Group("/")
	protected.Use(authMiddleware)

	RegisterAuthRoutes(auth, authHandler)
	RegisterPasswordResetRoutes(auth, passwordResetHandler)
	RegisterUserRoutes(protected, userHandler)
	RegisterListingsRoutes(api, protected, listingHandler)
	RegisterImageRoutes(api, imageHandler)
	RegisterOrderRoutes(protected, orderHandler)
	RegisterChatRoutes(protected, chatHandler)

	return router
}
