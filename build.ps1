$Name="G-itemViewer"
echo "Building for Linux..."
$env:GOOS="linux"; go build -o bin/${Name}-linux .
echo "Building for Mac..."
$env:GOOS="darwin"; go build -o bin/${Name}-mac .
echo "Building for Windows..."
$env:GOOS="windows"; go build -o bin/${Name}-win.exe -ldflags="-H=windowsgui" .
echo "Build complete."