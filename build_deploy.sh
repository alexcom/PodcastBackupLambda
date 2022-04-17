#!/usr/bin/env bash

rm -rf ./target || echo no target directory found
rm podcast_backup_lambda.zip || echo no deploy zip found
mkdir ./target
GOOS=linux go build -o ./target/podcast_backup podcast_backup.go

# as advised in https://aws.amazon.com/premiumsupport/knowledge-center/lambda-deployment-package-errors/
chmod 644 "$(find ./target -type f)"
chmod 755 "$(find ./target -type d)"

pushd target
zip -r ../podcast_backup_lambda.zip *
popd
aws lambda update-function-code --function-name PodcastBackup --zip-file fileb://./podcast_backup_lambda.zip --profile alexcom
