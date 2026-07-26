package app

import (
	"github.com/Jcorrieri/uf-marketplace/backend/config"
	"github.com/Jcorrieri/uf-marketplace/backend/handlers"
	"github.com/Jcorrieri/uf-marketplace/backend/middleware"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
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
	protected.POST("/listings/:id/publish", listingHandler.PublishListing)
	protected.DELETE("/listings/:id", listingHandler.DeleteListing)
}

func RegisterImageRoutes(
	public *gin.RouterGroup,
	protected *gin.RouterGroup,
	imageHandler *handlers.ImageHandler,
) {
	public.GET("/images/:imageId", imageHandler.GetImage)
	protected.POST("/images/uploads", imageHandler.BeginUpload)
	protected.POST("/images/:imageId/complete", imageHandler.CompleteUpload)
	protected.GET("/images/:imageId/status", imageHandler.GetStatus)
	protected.DELETE("/images/:imageId", imageHandler.DeleteImage)
}

func RegisterOrderRoutes(protected *gin.RouterGroup, orderHandler *handlers.OrderHandler) {
	protected.POST("/orders", orderHandler.CreateOrder)
	protected.GET("/orders/me", orderHandler.GetMyOrders)
}

func RegisterChatRoutes(
	protected *gin.RouterGroup,
	chatHandler *handlers.ChatHandler,
	chatWebSocketHandler *handlers.ChatWebSocketHandler,
) {
	protected.POST("/conversations", chatHandler.StartConversation)
	protected.GET("/conversations", chatHandler.GetConversations)
	protected.GET("/conversations/:id/messages", chatHandler.GetMessages)
	protected.GET("/ws/chat/:id", chatWebSocketHandler.Serve)
}

// NewRouter constructs the HTTP application from explicit dependencies.
func NewRouter(db *gorm.DB, configuration config.Config) *gin.Engine {
	return NewRouterWithObjectStore(
		db,
		configuration,
		services.UnavailableObjectStore{},
	)
}

// NewRouterWithObjectStore constructs the application with an injected image store.
func NewRouterWithObjectStore(
	db *gorm.DB,
	configuration config.Config,
	objectStore services.ObjectStore,
) *gin.Engine {
	authService := services.NewAuthService(db, configuration.JWTSecret)
	passwordResetService := services.NewPasswordResetService(db)
	userService := services.NewUserService(db)
	listingService := services.NewListingService(db)
	imageService := services.NewImageService(
		db,
		objectStore,
		utils.NewStandardImageVerifier(4096, 4096, 16*1024*1024),
	)
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
	chatHandler := handlers.NewChatHandler(chatService)
	chatWebSocketHandler := handlers.NewChatWebSocketHandler(chatService, hub)

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
	RegisterImageRoutes(api, protected, imageHandler)
	RegisterOrderRoutes(protected, orderHandler)
	RegisterChatRoutes(protected, chatHandler, chatWebSocketHandler)

	go hub.Run()
	return router
}
