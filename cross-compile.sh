echo "Start build for Linux (amd64)"
docker run --rm -v $(pwd):/app -w /app -e GOOS=linux -e GOARCH=amd64  golang:1.24 go build -o faf-pioneer-linux-amd64

echo "Start build for MacOS (arm64)"
docker run --rm -v $(pwd):/app -w /app -e GOOS=darwin -e GOARCH=arm64 golang:1.24 go build -o faf-pioneer-darwin-arm64

echo "Start build for Windows (amd64)"
docker run --rm -v $(pwd):/app -w /app -e GOOS=windows -e GOARCH=amd64 golang:1.24 go build -o faf-pioneer-windows-x64
