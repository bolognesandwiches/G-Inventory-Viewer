$Name = "G-itemViewer"
$ProjectPath = $PSScriptRoot
$GEarthExtPath = "C:\Users\atwym\Downloads\Windows.x64 (1)\Extensions\G-itemViewer"

# Ensure we're in the correct directory
Set-Location $ProjectPath

# Create bin directory if it doesn't exist
New-Item -ItemType Directory -Force -Path "bin"

# Create icon.rc file
@"
IDI_ICON1 ICON "assets/app_icon.ico"
"@ | Out-File -FilePath "icon.rc" -Encoding ascii

echo "Building for Windows..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# Generate the syso file from the rc file
windres -i icon.rc -o icon.syso -O coff

# Build the executable with the icon
go build -o "bin/${Name}-win.exe" -ldflags="-H=windowsgui -X main.Version=$(git describe --tags --always --dirty)" .

# Remove the temporary files
Remove-Item icon.rc
Remove-Item icon.syso

echo "Build complete."

# Create distribution folder
$distPath = "dist/windows"
New-Item -ItemType Directory -Force -Path $distPath

# Copy the executable
Copy-Item -Path "bin/${Name}-win.exe" -Destination "$distPath/${Name}.exe" -Force

# Copy to G-Earth extensions directory
echo "Copying to G-Earth extensions directory..."
New-Item -ItemType Directory -Force -Path $GEarthExtPath
Copy-Item -Path "$distPath/${Name}.exe" -Destination $GEarthExtPath -Force

echo "Build and installation complete."

# Create a batch file to run the executable (optional, for testing)
@"
@echo off
cd /d "%~dp0"
start "" "${Name}.exe"
"@ | Out-File -FilePath "$GEarthExtPath/Run_${Name}.bat" -Encoding ascii

echo "Created run batch file in G-Earth extensions directory."

# Optionally, create a ZIP file for distribution
$zipPath = "dist/${Name}_$(git describe --tags --always --dirty).zip"
Compress-Archive -Path "$distPath/*" -DestinationPath $zipPath -Force

echo "Created ZIP file for distribution: $zipPath"