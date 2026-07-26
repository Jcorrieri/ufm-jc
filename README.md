# UF Marketplace (Final Name TBD)

UF student marketplace platform project for CEN5035 - Software Engineering. Our application is intended to be a platform for students at the University of Florida to exchange goods and services. Users will ideally be able to upload listings, search and view existing listings using filters/categories, access a dashboard view to manage their posts, and possibly engage in real-time messaging with sellers. Users will need to sign up with their ufl email. 

## Members
- Shakir Gamzaev (Frontend)
- Pranav Padmapada Kodihalli (Frontend/Backend)
- Jacomo Corrieri (Frontend/Backend)
- Venkata Nitchaya Reddy Konkala (Backend)

## Running Locally

Currently, the UF Marketplace application is only available locally as a demo. 

### Installation

1.) Clone the repository: 

```
$ git clone https://github.com/Jcorrieri/uf-marketplace.git
$ cd uf-marketplace
```

2.) Install frontend dependencies

We use Angular as our frontend framework and manage dependencies using npm.

```
$ cd frontend
$ npm install
```

### Preparing and Starting the Backend

Our backend uses Gin for the API, GORM for database access, Gorilla for WebSockets, and SQLite for
local persistence. SQLite does not replace normal application and host security controls.

1.) Copy the example environment file `.example.env` → `.env`

(Working Directory: uf-marketplace/backend)

```
$ cd ../backend
$ cp .example.env .env
```

Replace the example JWT secret before running the application outside local development.

2.) Create or update the database schema

```
$ go run ./cmd/migrate
```

3.) Optionally seed local sample data

This creates image-free development listings. Object-backed seed images will be added with the
object-store adapter.

```
$ go run ./cmd/seed
```

4.) Start the API server

```
$ go run ./cmd/server
```

### Starting the Frontend

1.) After the dependencies are installed via npm the frontend can be started.

Working Directory: uf-marketplace/frontend

(In another terminal)

```
$ cd ../frontend
$ ng serve
```

## Using the application.

Sign in with the local seed account (`test@ufl.edu`, password `password`) or create an account.
Seed credentials are for local development only and must not be used in a deployed environment.
