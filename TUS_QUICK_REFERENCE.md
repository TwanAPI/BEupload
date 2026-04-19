# TUS Integration Quick Reference

Quick reference for developers implementing TUS and chunked uploads.

---

## 🚀 Quick Start

### Step 1: Install Dependencies
```bash
cd backend-go
go mod download
go mod tidy
```

### Step 2: Start Server
```bash
go run cmd/main.go
```

### Step 3: Test Upload
```bash
# Get token (replace with real payload)
TOKEN=$(curl -X POST http://localhost:3001/api/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}' | jq -r '.token')

# Upload file
curl -X POST http://localhost:3001/api/tiles/v2/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "chunk=@testfile.txt" \
  -F "uploadId=test-123" \
  -F "chunkIndex=0" \
  -F "totalChunks=1" \
  -F "fileName=testfile.txt" \
  -F "fileSize=$(stat -c%s testfile.txt)"
```

---

## 📋 API Quick Reference

### Endpoint Map
```
POST   /api/tiles              → Initiate TUS upload
PATCH  /api/tiles/:uploadId    → Upload TUS chunk  
HEAD   /api/tiles/:uploadId    → Get TUS offset
POST   /api/tiles/v2/upload    → Enhanced chunked (recommended)
GET    /api/tiles/:uploadId/status → Get progress
DELETE /api/tiles/:uploadId    → Cancel upload
GET    /api/tiles/:uploadId/resume → Resume info
```

### Required Headers
```
Authorization: Bearer TOKEN  (all endpoints)
Upload-Length: BYTES         (TUS - total size)
Upload-Offset: BYTES         (TUS - current offset)
Content-Type: application/offset+octet-stream  (TUS PATCH)
```

### Required Form Fields (Chunked V2)
```
chunk           - Binary chunk data
uploadId        - Upload session ID
chunkIndex      - 0-based chunk number
totalChunks     - Total number of chunks
fileName        - Original filename
fileSize        - Total file size in bytes
```

---

## 🔧 Implementation Checklist

### Server Setup
- [ ] Go 1.21+
- [ ] MongoDB connection working
- [ ] .env file configured
- [ ] JWT secret set
- [ ] `go mod download` executed
- [ ] `go build` successful

### API Testing
- [ ] Test standard upload `/api/files/upload`
- [ ] Test chunked v2 upload `/api/tiles/v2/upload`
- [ ] Test progress endpoint
- [ ] Test TUS initiation
- [ ] Test TUS chunk upload
- [ ] Test cancel endpoint

### Frontend Integration
- [ ] Import upload library (tus-js-client or fetch)
- [ ] Implement progress tracking
- [ ] Implement error handling
- [ ] Implement retry logic
- [ ] Test with real files
- [ ] Test network failure scenario

### Production Ready
- [ ] HTTPS enabled
- [ ] Rate limiting configured
- [ ] File size limits validated
- [ ] Upload cleanup scheduled
- [ ] Logging implemented
- [ ] Monitoring alerts set

---

## 💻 Code Examples

### Minimal Chunked Upload (JavaScript)
```javascript
async function uploadChunk(file, index, total, token) {
  const chunkSize = 5 * 1024 * 1024; // 5MB
  const start = index * chunkSize;
  const end = Math.min(start + chunkSize, file.size);
  const chunk = file.slice(start, end);
  
  const form = new FormData();
  form.append('chunk', chunk);
  form.append('uploadId', 'upload-' + Date.now());
  form.append('chunkIndex', index);
  form.append('totalChunks', total);
  form.append('fileName', file.name);
  form.append('fileSize', file.size);
  
  const res = await fetch('/api/tiles/v2/upload', {
    method: 'POST',
    headers: { 'Authorization': `Bearer ${token}` },
    body: form
  });
  
  return res.json();
}
```

### Get Upload Status (JavaScript)
```javascript
async function getProgress(uploadId, token) {
  const res = await fetch(`/api/files/upload-progress/${uploadId}`, {
    headers: { 'Authorization': `Bearer ${token}` }
  });
  const data = await res.json();
  console.log(`Progress: ${data.progress}%`);
  return data;
}
```

### TUS Upload with Resume (cURL)
```bash
#!/bin/bash
FILE=$1
TOKEN=$2
UPLOAD_ID="upload-$(date +%s)"

# Step 1: Initiate
LOCATION=$(curl -X POST http://localhost:3001/api/tiles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Upload-Length: $(stat -c%s $FILE)" \
  -H "Content-Length: 0" \
  -s -i | grep Location | cut -d' ' -f2)

echo "Upload ID: $UPLOAD_ID"
echo "Location: $LOCATION"

# Step 2: Check offset before uploading
OFFSET=$(curl -I "$LOCATION" \
  -H "Authorization: Bearer $TOKEN" \
  -s | grep Upload-Offset | cut -d' ' -f2)

echo "Current offset: $OFFSET"

# Step 3: Upload chunk
curl -X PATCH "$LOCATION" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Upload-Offset: $OFFSET" \
  -H "Content-Type: application/offset+octet-stream" \
  --data-binary "@$FILE"

# Step 4: Check final status
curl "$LOCATION/status" \
  -H "Authorization: Bearer $TOKEN" \
  -s | jq '.'
```

### Python Multi-Chunk Upload
```python
import requests
import os

def upload_chunked(file_path, api_url, token, chunk_size=5*1024*1024):
    file_size = os.path.getsize(file_path)
    upload_id = f"py-{int(time.time())}"
    chunks = (file_size + chunk_size - 1) // chunk_size
    
    with open(file_path, 'rb') as f:
        for i in range(chunks):
            chunk = f.read(chunk_size)
            files = {'chunk': chunk}
            data = {
                'uploadId': upload_id,
                'chunkIndex': i,
                'totalChunks': chunks,
                'fileName': os.path.basename(file_path),
                'fileSize': file_size
            }
            
            r = requests.post(
                f'{api_url}/api/tiles/v2/upload',
                files=files,
                data=data,
                headers={'Authorization': f'Bearer {token}'}
            )
            
            result = r.json()
            print(f"[{i+1}/{chunks}] Progress: {result.get('progress')}%")
            
            if r.status_code != 200:
                print(f"Error: {result}")
                return False
    
    return True
```

---

## 🧪 Testing Scenarios

### Test 1: Single Chunk Upload
```bash
# Create small test file
echo "test data" > test.txt

# Upload
curl -F "chunk=@test.txt" \
     -F "uploadId=test1" \
     -F "chunkIndex=0" \
     -F "totalChunks=1" \
     -F "fileName=test.txt" \
     -F "fileSize=10" \
     -H "Authorization: Bearer $TOKEN" \
     http://localhost:3001/api/tiles/v2/upload | jq '.'
```

### Test 2: Multi-Chunk Upload
```bash
# Create 10MB test file
dd if=/dev/urandom of=large.bin bs=1M count=10

# Upload in 2MB chunks
CHUNK_SIZE=$((2 * 1024 * 1024))
FILE_SIZE=$(stat -c%s large.bin)
TOTAL_CHUNKS=$((($FILE_SIZE + $CHUNK_SIZE - 1) / $CHUNK_SIZE))

for i in $(seq 0 $((TOTAL_CHUNKS - 1))); do
  echo "Uploading chunk $((i+1))/$TOTAL_CHUNKS"
  dd if=large.bin bs=$CHUNK_SIZE skip=$i count=1 2>/dev/null | \
  curl -F "chunk=@-" \
       -F "uploadId=large1" \
       -F "chunkIndex=$i" \
       -F "totalChunks=$TOTAL_CHUNKS" \
       -F "fileName=large.bin" \
       -F "fileSize=$FILE_SIZE" \
       -H "Authorization: Bearer $TOKEN" \
       http://localhost:3001/api/tiles/v2/upload
done
```

### Test 3: TUS Protocol Upload
```bash
# Initiate
RESP=$(curl -X POST http://localhost:3001/api/tiles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Upload-Length: 1000000" \
  -H "Content-Length: 0" \
  -i)

LOCATION=$(echo "$RESP" | grep Location | cut -d' ' -f2)
echo "Upload Location: $LOCATION"

# Upload 500KB chunk
dd if=/dev/urandom of=chunk.bin bs=1K count=500
curl -X PATCH "$LOCATION" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Upload-Offset: 0" \
  -H "Content-Type: application/offset+octet-stream" \
  --data-binary @chunk.bin
```

### Test 4: Error Handling
```bash
# Missing token (should get 401)
curl -X POST http://localhost:3001/api/tiles/v2/upload \
  -F "chunk=@test.txt" 2>&1 | grep -i unauthorized

# Invalid uploadId format (should get 400)
curl -X POST http://localhost:3001/api/tiles/v2/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "uploadId=invalid"

# Check nonexistent upload (should get 404)
curl http://localhost:3001/api/tiles/nonexistent/status \
  -H "Authorization: Bearer $TOKEN"
```

---

## 📊 Monitoring

### Check Running Uploads
```bash
# View uploads directory
du -sh backend-go/uploads/

# List temp chunks
ls -la backend-go/uploads/.chunks/

# Count active uploads
ls backend-go/uploads/.tus-data/ | wc -l
```

### Monitor Server Logs
```bash
# Tail logs
tail -f logs/server.log | grep -i upload

# Search errors
grep -i "error\|failed" logs/server.log | tail -20
```

### Performance Testing
```bash
# Test throughput with Apache Bench
ab -n 100 -c 4 http://localhost:3001/api/health

# Stress test with wrk
wrk -t4 -c100 -d30s http://localhost:3001/api/files
```

---

## 🔍 Debugging

### Enable Debug Logging
```go
// In backend-go/cmd/main.go
router.Use(gin.Logger())
router.Use(gin.Recovery())
```

### Common Issues

**Issue: "Upload handler initialization failed"**
```
Solution: Check uploads/ directory exists and is writable
chmod 755 backend-go/uploads/
```

**Issue: "Chunk not found when finalizing"**
```
Solution: Verify chunkIndex matches order, check file permissions
ls -la backend-go/uploads/.chunks/UPLOAD_ID*
```

**Issue: "Memory allocation failed"**
```
Solution: Check chunk size, reduce CHUNK_MAX_SIZE in env
export CHUNK_MAX_SIZE=52428800  # 50MB instead of 100MB
```

**Issue: "Connection timeout on large file"**
```
Solution: Increase timeout, use smaller chunks
export HTTP_TIMEOUT=300
```

---

## 📦 Dependencies

### Required Packages
```go
github.com/tus/tusd/v2 v2.4.0           // TUS protocol
github.com/google/uuid v1.5.0             // UUID generation  
github.com/gin-gonic/gin v1.9.1           // Web framework
go.mongodb.org/mongo-driver v1.12.1       // Database
github.com/golang-jwt/jwt/v5 v5.0.0       // Authentication
```

### Install
```bash
go mod download
go mod tidy
```

---

## 📝 Notes

- All endpoints require valid JWT token (login first)
- Uploads expire after 24 hours
- Max file size: 5GB
- Temp files cleaned up every hour
- All endpoints rate-limited to 1500 req/min
- MongoDB required for file metadata storage

---

## ✅ Deployment Checklist

- [ ] Dependencies installed (`go mod download`)
- [ ] Binary compiled (`go build`)
- [ ] Server started and responding
- [ ] All endpoints accessible
- [ ] Authentication working
- [ ] Database connected
- [ ] TUS handler initialized
- [ ] Upload directory writable
- [ ] Cleanup tasks scheduled
- [ ] Monitoring configured
- [ ] Tests passed

---

## Next Steps

1. ✅ Test all endpoints with provided examples
2. ✅ Integrate frontend uploader
3. ✅ Deploy to staging
4. ✅ Load testing
5. ✅ Production deployment
