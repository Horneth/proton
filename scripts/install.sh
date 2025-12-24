#!/bin/bash
# Proton Installation & Setup Script
set -e

PROTON_DIR="$HOME/.proton"
BIN_INSTALL_PATH="/usr/local/bin/proton"

echo "🚀 Setting up Proton..."

# 1. Create Proton directory
mkdir -p "$PROTON_DIR"

# 2. Install Binary (if present in current directory)
if [ -f "./proton" ]; then
    echo "📦 Installing 'proton' binary to $BIN_INSTALL_PATH..."
    if [ -w "/usr/local/bin" ]; then
        cp ./proton "$BIN_INSTALL_PATH"
    else
        echo "Permission denied for /usr/local/bin, using sudo..."
        sudo cp ./proton "$BIN_INSTALL_PATH"
    fi
    chmod +x "$BIN_INSTALL_PATH"
else
    echo "⚠️  'proton' binary not found in current directory. Skipping binary installation."
    echo "   (Make sure 'proton' is in your PATH manually if you skip this)."
fi

# 3. Install Default Config
if [ -f "./.default.proto.config.json" ]; then
    echo "⚙️  Installing default config to $PROTON_DIR/config.json..."
    cp "./.default.proto.config.json" "$PROTON_DIR/config.json"
elif [ -f ".default.proto.config.json" ]; then
    cp ".default.proto.config.json" "$PROTON_DIR/config.json"
fi

# 4. Install Default Buf Image
if [ -f "./canton_buf_image.binpb" ]; then
    echo "🖼️  Installing consolidated Buf image to $PROTON_DIR/proton.binpb..."
    cp "./canton_buf_image.binpb" "$PROTON_DIR/proton.binpb"
elif [ -f "canton_buf_image.binpb" ]; then
    cp "canton_buf_image.binpb" "$PROTON_DIR/proton.binpb"
fi

echo ""
echo "✅ Proton setup complete!"
echo "   Config: $PROTON_DIR/config.json"
echo "   Image:  $PROTON_DIR/proton.binpb"
echo ""
echo "Try running: proton canton topology template TopologyTransaction"
