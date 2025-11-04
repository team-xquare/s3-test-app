package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	s3Client   *s3.Client
	bucketName string
)

func main() {
	endpoint := getEnv("S3_ENDPOINT", "http://xquare-test:8333")
	accessKey := getEnv("S3_ACCESS_KEY", "")
	secretKey := getEnv("S3_SECRET_KEY", "")
	bucketName = getEnv("S3_BUCKET", "test-buckets")
	region := getEnv("S3_REGION", "us-east-1")
	port := getEnv("PORT", "8080")

	if accessKey == "" || secretKey == "" {
		log.Fatal("S3_ACCESS_KEY and S3_SECRET_KEY must be set")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/list", handleList)
	http.HandleFunc("/download", handleDownload)

	log.Printf("Starting S3 test server on port %s", port)
	log.Printf("S3 Endpoint: %s", endpoint)
	log.Printf("S3 Bucket: %s", bucketName)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>S3 Test App</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .section { margin: 30px 0; padding: 20px; border: 1px solid #ddd; border-radius: 5px; }
        button { padding: 10px 20px; background: #007bff; color: white; border: none; cursor: pointer; border-radius: 3px; }
        button:hover { background: #0056b3; }
        input[type="file"] { margin: 10px 0; }
        #result { margin-top: 20px; padding: 10px; background: #f8f9fa; border-radius: 3px; }
        .file-list { list-style: none; padding: 0; }
        .file-list li { padding: 10px; margin: 5px 0; background: #f8f9fa; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>S3 Test Application</h1>
    
    <div class="section">
        <h2>Upload File</h2>
        <input type="file" id="fileInput">
        <button onclick="uploadFile()">Upload</button>
    </div>
    
    <div class="section">
        <h2>List Files</h2>
        <button onclick="listFiles()">Refresh List</button>
        <ul id="fileList" class="file-list"></ul>
    </div>
    
    <div id="result"></div>
    
    <script>
        async function uploadFile() {
            const fileInput = document.getElementById('fileInput');
            const file = fileInput.files[0];
            if (!file) {
                alert('Please select a file');
                return;
            }
            
            const formData = new FormData();
            formData.append('file', file);
            
            try {
                const response = await fetch('/upload', {
                    method: 'POST',
                    body: formData
                });
                const data = await response.json();
                document.getElementById('result').innerHTML = 
                    '<strong>Upload Result:</strong><br>' + JSON.stringify(data, null, 2);
                listFiles();
            } catch (error) {
                document.getElementById('result').innerHTML = 
                    '<strong>Error:</strong> ' + error.message;
            }
        }
        
        async function listFiles() {
            try {
                const response = await fetch('/list');
                const data = await response.json();
                const fileList = document.getElementById('fileList');
                
                if (data.files && data.files.length > 0) {
                    fileList.innerHTML = data.files.map(file => 
                        '<li>' + file.key + ' (' + formatBytes(file.size) + ') - ' + 
                        '<a href="/download?key=' + encodeURIComponent(file.key) + '">Download</a></li>'
                    ).join('');
                } else {
                    fileList.innerHTML = '<li>No files found</li>';
                }
            } catch (error) {
                document.getElementById('result').innerHTML = 
                    '<strong>Error:</strong> ' + error.message;
            }
        }
        
        function formatBytes(bytes) {
            if (bytes === 0) return '0 Bytes';
            const k = 1024;
            const sizes = ['Bytes', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
        }
        
        window.onload = listFiles;
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	size, err := io.Copy(buf, file)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	key := fmt.Sprintf("%d-%s", time.Now().Unix(), header.Filename)
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"key":      key,
		"size":     size,
		"filename": header.Filename,
	})
}

func handleList(w http.ResponseWriter, r *http.Request) {
	result, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	files := make([]map[string]interface{}, 0)
	for _, obj := range result.Contents {
		files = append(files, map[string]interface{}{
			"key":          *obj.Key,
			"size":         *obj.Size,
			"lastModified": obj.LastModified.Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"files": files,
		"count": len(files),
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key parameter required", http.StatusBadRequest)
		return
	}

	result, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer result.Body.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", key))
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, result.Body)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
