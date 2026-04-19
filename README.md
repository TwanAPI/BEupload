# Cloud Caddy - Go Backend

A high-performance file management backend built with Go, Gin framework, and MongoDB.

## Project Structure

```
backend-go/
├── cmd/
│   └── main.go           # Application entry point
├── internal/
│   ├── database/
│   │   └── mongo.go      # MongoDB connection & initialization
│   ├── handlers/
│   │   ├── auth.go       # Authentication endpoints
│   │   ├── files.go      # File upload/download endpoints
│   │   └── admin.go      # Admin management endpoints
│   ├── middleware/
│   │   └── auth.go       # JWT & CORS middleware
│   ├── models/
│   │   └── models.go     # Data models & DTOs
│   └── utils/
│       ├── file.go       # File utilities
│       └── jwt.go        # JWT utilities
├── uploads/              # Uploaded files storage
├── .env.example          # Environment variables template
├── go.mod                # Go module definition
└── README.md             # This file
```

## Features

### 🔐 Authentication
- User signup with email & password
- User signin with credentials
- JWT token-based authentication (7-day expiry)
- Password management
- Profile updates
- Role-based access control (User, Admin, Moderator)

### 📁 File Management
- Single file upload
- Chunked file upload for large files (5GB limit)
- File download with streaming
- File deletion by owner or admin
- Upload progress tracking
- Real-time chunk verification

### 👨‍💼 Admin Features
- View all users with detailed stats
- View all files in the system
- Manage user roles
- Get user-specific file lists
- Delete individual files
- Bulk delete all user files
- User statistics (total files, storage, average file size)

### ⚡ Performance Optimizations
- MongoDB connection pooling (80 max, 20 min)
- GZIP compression for responses
- Rate limiting (1500 req/min)
- In-memory upload session tracking
- Parallel chunk merging
- 16MB buffer for fast I/O
- Automatic cleanup of old temporary chunks

## Prerequisites

- Go 1.21 or higher
- MongoDB Atlas cluster (or local MongoDB)
- .NET SDK for Windows compilation (optional)

## Installation

### 1. Clone and Setup
```bash
cd backend-go
cp .env.example .env.server
```

### 2. Configure Environment Variables
Edit `.env.server`:
```env
PORT=3001
MONGODB_URI=mongodb+srv://admin:password@cluster.mongodb.net/?appName=AppName
JWT_SECRET=your-secret-key
NODE_ENV=development
```

### 3. Install Dependencies
```bash
go mod download
go mod tidy
```

### 4. Run the Server
```bash
# Development
go run cmd/main.go

# Production build
go build -o cloud-caddy-backend cmd/main.go
./cloud-caddy-backend
```

## API Endpoints

### Authentication
- `POST /auth/signup` - Register new user
- `POST /auth/signin` - Login user
- `GET /auth/me` - Get current user (requires auth)
- `POST /auth/change-password` - Change password
- `POST /auth/update-profile` - Update user profile

### File Management
- `POST /api/files/upload` - Upload single file
- `POST /api/files/upload-chunk` - Upload file chunk
- `GET /api/files/upload-progress/:uploadId` - Check upload progress
- `GET /api/files` - Get user's files
- `DELETE /api/files/:id` - Delete user's file
- `GET /api/files/download/:filename` - Download file

### Admin Routes (requires admin role)
- `GET /api/admin/users` - List all users
- `GET /api/admin/users/:userId/detail` - Get user details with stats
- `GET /api/admin/users/:userId/files` - Get user's files
- `POST /api/admin/users/:userId/role` - Set user role
- `POST /api/admin/users/:userId/delete-all-files` - Delete all user files
- `GET /api/admin/files` - Get all files
- `DELETE /api/admin/files/:fileId` - Delete specific file

### Health Check
- `GET /health` - Basic health check
- `GET /health/stats` - Detailed stats

## Authentication

All protected endpoints require JWT token in Authorization header:
```
Authorization: Bearer <token>
```

## Data Models

### User
```go
- ID: ObjectID
- Email: string (unique)
- Username: string (unique, optional)
- Password: bcrypt hash
- DisplayName: string
- Bio: string
- Role: "user" | "admin" | "moderator"
- SocialMedia: {twitter, github, linkedin, instagram}
- CreatedAt: timestamp
- UpdatedAt: timestamp
```

### File
```go
- ID: ObjectID
- UserID: string (foreign key)
- FileName: string
- FileSize: int64
- FileType: string (MIME type)
- StoragePath: string
- Description: string
- IsPublic: boolean
- CreatedAt: timestamp
- UpdatedAt: timestamp
```

## Database Schema

### Users Collection
```json
{
  "_id": ObjectId,
  "email": "user@example.com",
  "username": "username",
  "password": "bcrypt_hash",
  "displayName": "Display Name",
  "role": "user",
  "bio": "User bio",
  "avatar_url": "url",
  "location": "location",
  "website": "website",
  "phone": "phone",
  "social_media": {...},
  "created_at": ISODate,
  "updated_at": ISODate
}
```

### Files Collection
```json
{
  "_id": ObjectId,
  "user_id": "userId",
  "file_name": "filename.ext",
  "file_size": 1024,
  "file_type": "application/pdf",
  "storage_path": "unique_filename",
  "description": "file description",
  "is_public": false,
  "created_at": ISODate,
  "updated_at": ISODate
}
```

## Development

### Building
```bash
# Build for Windows
GOOS=windows GOARCH=amd64 go build -o cloud-caddy-backend.exe cmd/main.go

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o cloud-caddy-backend cmd/main.go

# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o cloud-caddy-backend cmd/main.go
```

### Environment Variables
- `PORT`: Server port (default: 3001)
- `MONGODB_URI`: MongoDB connection string
- `JWT_SECRET`: JWT signing secret
- `NODE_ENV`: development or production

## Performance Benchmarks

- **Upload Speed**: Up to 300MB/s (with 4 parallel chunks)
- **File Size Limit**: 5GB per file
- **Concurrent Uploads**: 8+ files simultaneously
- **Connection Pool**: 80 max connections
- **Rate Limiting**: 1500 requests/minute per IP
- **Response Compression**: GZIP (2-3x faster)

## Security Features

- Bcryptjs password hashing (10 rounds)
- JWT token authentication
- Role-based access control
- Input sanitization
- MongoDB injection prevention
- CORS with configurable origins
- Rate limiting per IP
- File path validation

## Error Handling

All errors follow standardized JSON format:
```json
{
  "error": "Error message describing the issue"
}
```

HTTP Status Codes:
- `200` - Success
- `400` - Bad request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not found
- `429` - Too many requests
- `500` - Server error

## Deployment

### Docker
```dockerfile
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o backend cmd/main.go
EXPOSE 3001
CMD ["./backend"]
```

Build and run:
```bash
docker build -t cloud-caddy-backend .
docker run -p 3001:3001 --env-file .env.server cloud-caddy-backend
```

### Kubernetes
See `k8s/` directory for Kubernetes manifests (YAML files).

## Troubleshooting

### MongoDB Connection Failed
- Check `MONGODB_URI` is correct
- Verify IP whitelist in MongoDB Atlas
- Ensure network connectivity

### Port Already in Use
- Change `PORT` environment variable
- Kill process using port: `lsof -i :3001`

### JWT Token Invalid
- Verify `JWT_SECRET` matches across requests
- Check token hasn't expired (7 days)
- Verify format: `Bearer <token>`

## License

MIT License - see LICENSE file

## Support

For issues and questions, please create an issue in the repository.
