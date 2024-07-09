$Name = "G-itemViewer"
$ProjectPath = $PSScriptRoot  # This will set the path to the directory containing the script
$ExtPath = (Get-Item $ProjectPath).Parent.FullName  # This will set the path to the parent directory (Ext)

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
go build -o "bin/${Name}-win.exe" -ldflags="-H=windowsgui" .

# Remove the temporary files
Remove-Item icon.rc
Remove-Item icon.syso

echo "Build complete."

# Copy necessary assets
echo "Copying assets..."

# Create assets directory in bin if it doesn't exist
New-Item -ItemType Directory -Force -Path "bin/assets"

# Copy scan icon and app icon
Copy-Item -Path "assets/scan_icon.png" -Destination "bin/assets/scan_icon.png" -Force
Copy-Item -Path "assets/app_icon.ico" -Destination "bin/assets/app_icon.ico" -Force

# Copy fonts
Copy-Item -Path "$ExtPath/Volter_Goldfish.ttf" -Destination "bin/Volter_Goldfish.ttf" -Force
Copy-Item -Path "$ExtPath/Volter_Goldfish_bold.ttf" -Destination "bin/Volter_Goldfish_bold.ttf" -Force

# Copy fonts to assets folder as well (for redundancy)
Copy-Item -Path "$ExtPath/Volter_Goldfish.ttf" -Destination "bin/assets/Volter_Goldfish.ttf" -Force
Copy-Item -Path "$ExtPath/Volter_Goldfish_bold.ttf" -Destination "bin/assets/Volter_Goldfish_bold.ttf" -Force

echo "Asset copying complete."

echo "Creating distribution folder..."

# Create distribution folder
$distPath = "dist/windows"
New-Item -ItemType Directory -Force -Path $distPath

# Copy the executable
Copy-Item -Path "bin/${Name}-win.exe" -Destination "$distPath/${Name}.exe" -Force

# Copy assets folder
Copy-Item -Path "bin/assets" -Destination "$distPath" -Recurse -Force

# Copy fonts to root of distribution folder
Copy-Item -Path "bin/Volter_Goldfish.ttf" -Destination "$distPath/Volter_Goldfish.ttf" -Force
Copy-Item -Path "bin/Volter_Goldfish_bold.ttf" -Destination "$distPath/Volter_Goldfish_bold.ttf" -Force

echo "Distribution folder created."