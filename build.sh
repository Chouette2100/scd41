#!/bin/bash

# 引数が渡されていない場合のチェック
if [ $# -eq 0 ]; then
  echo "エラー: 引数が必要です。"
  echo "使用方法: $0 ロードモジュール名"
  exit 1  # エラーコード1で終了
fi

# 引数が渡されている場合の処理
echo "引数の数: $#"
echo "すべての引数: $@"

export GOOS=linux
export GOARCH=arm64
export CC=aarch64-linux-gnu-gcc
export CGO_ENABLED=1
go build -o $1

# 変数の設定
export LOCAL_FILE=$1
export REMOTE_USER=orangepi
export REMOTE_IP=192.168.0.16
export REMOTE_DIR=/home/orangepi/MyProject/Measurements/$1
export REMOTE_FILE=$REMOTE_DIR/$LOCAL_FILE

# ファイルのコピー

# capabilityの設定
scp ./$1 $REMOTE_USER@$REMOTE_IP:$REMOTE_DIR && \
ssh $REMOTE_USER@$REMOTE_IP "sudo -S setcap cap_sys_rawio,cap_dac_override+ep $REMOTE_FILE"
