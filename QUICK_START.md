# Go Backend - Quick Start Guide

Get the Cloud Caddy Go backend running in minutes!

## Option 1: Quick Setup (Recommended for Development)

### Prerequisites
- Go 1.21+ installed ([download](https://golang.org/dl/))
- MongoDB Atlas account (free tier available at [mongodb.com/cloud/atlas](https://mongodb.com/cloud/atlas))
- Git

### Steps

1. **Clone and navigate**
   ```bash
   cd backend-go
   ```

2. **Setup environment file**
   ```bash
   cp .env.example .env.server
   ```

3. **Edit `.env.server`**
   ```env
   PORT=3001
   MONGODB_URI=mongodb+srv://admin:password@cluster.mongodb.net/?appName=UserUpload
   JWT_SECRET=your-secret-key
   NODE_ENV=development
   ```

4. **Install Go dependencies**
   ```bash
   go mod download
   ```

5. **Run the server**
   ```bash
   go run cmd/main.go
   ```

   You should see:
   ```
   🔄 Connecting to MongoDB...
   ✅ MongoDB connected successfully
   ✅ Server starting on http://localhost:3001
   ```

6. **Test the API**
   ```bash
   curl http://localhost:3001/health
   ```
   
   Expected response:
   ```json
   {"status":"ok"}
   ```

---

## Option 2: Docker Setup (Recommended for Production)

### Prerequisites
- Docker installed ([download](https://www.docker.com/products/docker-desktop))
- Docker Compose installed

### Steps

1. **Navigate to backend-go directory**
   ```bash
   cd backend-go
   ```

2. **Create `.env.server` (optional)**
   ```bash
   cp .env.example .env.server
   ```

3. **Start with Docker Compose**
   ```bash
   docker-compose up --build
   ```

   This will:
   - Build the Go backend image
   - Start MongoDB container locally
   - Start the backend server on port 3001

4. **Test the API**
   ```bash
   curl http://localhost:3001/health
   ```

5. **Stop the containers**
   ```bash
   docker-compose down
   ```

---

## Option 3: Build Standalone Binary

### Windows
```bash
GOOS=windows GOARCH=amd64 go build -o cloud-caddy-backend.exe cmd/main.go
./cloud-caddy-backend.exe
```

### Linux
```bash
GOOS=linux GOARCH=amd64 go build -o cloud-caddy-backend cmd/main.go
./cloud-caddy-backend
```

### macOS
```bash
GOOS=darwin GOARCH=amd64 go build -o cloud-caddy-backend cmd/main.go
./cloud-caddy-backend
```

---

## API Testing

### 1. Register a User
```bash
curl -X POST http://localhost:3001/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "displayName": "John Doe",
    "username": "johndoe"
  }'
```

Response:
```json
{
  "user": {
    "id": "user_id",
    "email": "user@example.com",
    "displayName": "John Doe",
    "username": "johndoe"
  },
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

### 2. Upload a File
```bash
curl -X POST http://localhost:3001/api/files/upload \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@/path/to/file.pdf" \
  -F "description=My PDF file"
```

### 3. Get User Files
```bash
curl http://localhost:3001/api/files \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 4. Admin: Get All Users
```bash
# First, set a user as admin via MongoDB or API
curl http://localhost:3001/api/admin/users \
  -H "Authorization: Bearer ADMIN_TOKEN"
```

---

## File Structure

```
backend-go/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── database/            # MongoDB setup
│   ├── handlers/            # Route handlers (auth, files, admin)
│   ├── middleware/          # JWT, CORS, rate limiting
│   ├── models/              # Data models
│   └── utils/               # Utilities
├── uploads/                 # File storage
├── .env.example             # Environment template
├── docker-compose.yml       # Docker setup
├── Dockerfile               # Docker image
├── go.mod/go.sum            # Dependencies
└── README.md                # Full documentation
```

---

## Common Issues & Solutions

### Issue: "Connection refused" on MongoDB
**Solution**: 
- Ensure MongoDB Atlas cluster is accessible
- Check IP whitelist in MongoDB Atlas
- Verify connection string in `.env.server`

### Issue: "Port 3001 already in use"
**Solution**:
```bash
# Change PORT in .env.server
PORT=3002

# Or kill the process (Linux/Mac):
lsof -i :3001
kill -9 <PID>
```

### Issue: "Cannot find go.mod"
**Solution**:
```bash
cd backend-go
go mod download
go mod tidy
```

### Issue: Docker build fails
**Solution**:
```bash
# Clean Docker cache and rebuild
docker-compose down -v
docker-compose up --build --force-recreate
```

---

## Development Workflow

### Live Reload (using fswatch or similar)
```bash
# Install air for hot reload (optional)
go install github.com/cosmtrek/air@latest

# Run with air
air
```

### Running Tests
```bash
go test ./...
```

### Code Formatting
```bash
go fmt ./...
```

### Dependency Management
```bash
# Add a new dependency
go get github.com/user/package

# Update dependencies
go get -u ./...

# Clean up unused dependencies
go mod tidy
```

---

## Performance Tips

1. **Use MongoDB Atlas M0 (free tier)** for development
2. **Enable compression** in Go for faster responses
3. **Use connection pooling** (already configured)
4. **Monitor upload progress** for large files
5. **Implement pagination** for file lists

---

## Security Checklist

- [ ] Set strong `JWT_SECRET`
- [ ] Use MongoDB Atlas with network restrictions
- [ ] Enable HTTPS in production
- [ ] Configure CORS properly
- [ ] Set up rate limiting
- [ ] Use environment variables for secrets
- [ ] Validate all user inputs
- [ ] Update dependencies regularly

---

## Next Steps

1. ✅ Backend running on `http://localhost:3001`
2. 📝 Review API documentation in [README.md](README.md)
3. 🔐 Set up authentication flow
4. 📁 Implement file upload/download
5. 👨‍💼 Configure admin dashboard
6. 🚀 Deploy to production

---

## Need Help?

- 📖 Check [README.md](README.md) for detailed documentation
- 💬 Review API examples in this guide
- 🐛 Debug with detailed logs: `RUST_LOG=debug`
- 📞 Create an issue in the repository

Happy coding! 🎉
