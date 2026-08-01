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

#### Optional: Use Amazon S3 for Images

SQLite continues to store image ownership, lifecycle state, and verified metadata. Image bytes
can be stored in a private S3 bucket while both application processes run locally.

1.) Create a private, nonversioned general-purpose S3 bucket. Keep Block Public Access enabled
and use the default S3-managed server-side encryption.

2.) Add this CORS configuration to the bucket, replacing the origin if Angular runs elsewhere:

```json
[
  {
    "AllowedHeaders": ["*"],
    "AllowedMethods": ["POST", "GET", "HEAD"],
    "AllowedOrigins": ["http://localhost:4200"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3000
  }
]
```

3.) Add a bucket lifecycle rule that expires objects under the `staging/` prefix after one day.
This is a safety net for uploads that are abandoned before verification or whose immediate
staging cleanup fails.

4.) Grant the AWS identity used by the backend the following least-privilege IAM policy. Replace
`YOUR_BUCKET_NAME` in both resource ARNs:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
      "Resource": [
        "arn:aws:s3:::YOUR_BUCKET_NAME/staging/*",
        "arn:aws:s3:::YOUR_BUCKET_NAME/images/*"
      ]
    }
  ]
}
```

5.) Authenticate locally with an AWS profile or standard AWS credential environment variables.
Do not put access keys in `backend/.env`. Configure only these non-secret values there:

```dotenv
OBJECT_STORE_PROVIDER="s3"
S3_BUCKET="YOUR_BUCKET_NAME"
AWS_REGION="us-east-1"
AWS_PROFILE="your-local-profile"
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
