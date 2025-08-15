#!/bin/bash

OS=$(uname -s)
ARCH=$(uname -m)

# Map aarch64 to arm64
[ "$ARCH" = "aarch64" ] && ARCH="arm64"

BASE_FILENAME="lsh_%s_%s"
FILENAME=$(printf "$BASE_FILENAME" "$OS" "$ARCH")

# The latest release will be the most recent non-prerelease, non-draft release.
LATEST=$(curl -L -s \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/latitudesh/lsh/releases/latest | grep "tag_name" | cut -d "\"" -f 4)

URL="https://github.com/latitudesh/lsh/releases/download/$LATEST/$FILENAME.tar.gz"

echo -e "[lsh] Download started!\n"
curl -L -o lsh.tar.gz $URL
echo -e "[lsh] Download finished!\n"

echo -e "[lsh] Setting up the CLI\n"
HOME_DIR=$(echo ~)
INSTALL_DIR="$HOME_DIR/.lsh"
mkdir -p $INSTALL_DIR
tar -xzf lsh.tar.gz
mv "$FILENAME/lsh" $INSTALL_DIR

# Detect the current shell and add the directory to the user's PATH
SHELL_NAME=$(basename "$SHELL")

SHELL_CONFIG_PATH=""

case "$SHELL_NAME" in
"bash")
  SHELL_CONFIG_PATH=~/.bashrc
  echo 'export PATH="$PATH:$HOME/.lsh"' >>$SHELL_CONFIG_PATH
  ;;
"zsh")
  SHELL_CONFIG_PATH=~/.zshrc
  echo 'export PATH="$PATH:$HOME/.lsh"' >>$SHELL_CONFIG_PATH
  ;;
"fish")
  SHELL_CONFIG_PATH=~/.config/fish/config.fish
  echo 'set -gx PATH $PATH $HOME_DIR/.lsh' >>$SHELL_CONFIG_PATH
  ;;
*)
  echo "Unsupported shell: $SHELL_NAME"
  ;;
esac

echo -e "[lsh] Removing installation files\n"
rm lsh.tar.gz
rm -rf $FILENAME

echo -e "[lsh] Installation finished! To enable the lsh command, run: \n"
echo -e "    source $SHELL_CONFIG_PATH \n"
