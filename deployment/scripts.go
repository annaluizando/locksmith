package deployment

import (
	"bytes"
	"fmt"
	"text/template"
)

// holds the configuration needed to generate a deployment script.
type ScriptData struct {
	Provider           string
	SecretID           string
	ProjectID          string
	Region             string
	VaultURI           string
	SecretName         string
	SentryDSN          string
	SlackBotToken      string
	SlackChannelID     string
	FunctionAppName    string
	StorageAccountName string
	ResourceGroupName  string
}

const (
	awsScriptTemplate = `#!/bin/bash
# Deployment script for AWS Lambda

echo "--- Building Go binary for Lambda ---"
GOOS=linux go build -o main deployment/aws/main.go

echo "--- Creating deployment package ---"
zip deployment.zip main

echo "--- Deploying to AWS ---"
# Note: This script assumes you have configured your AWS CLI and have the necessary permissions.

# You may need to create an IAM role with permissions for Secrets Manager, CloudWatch Logs,
# and give EventBridge permissions to invoke this Lambda.
# Replace this with the ARN of the role you create.
IAM_ROLE_ARN="REPLACE_WITH_YOUR_LAMBDA_EXECUTION_ROLE_ARN"
FUNCTION_NAME="jwtSecretRotator"
SCHEDULE="rate(24 hours)"

aws lambda create-function \
  --function-name "$FUNCTION_NAME" \
  --runtime go1.x \
  --role "$IAM_ROLE_ARN" \
  --handler main \
  --zip-file fileb://deployment.zip \
  --environment "Variables={SECRET_ID={{.SecretID}},REGION={{.Region}},SENTRY_DSN={{.SentryDSN}},SLACK_BOT_TOKEN={{.SlackBotToken}},SLACK_CHANNEL_ID={{.SlackChannelID}}}"

echo "--- Creating EventBridge rule for scheduled rotation ---"
RULE_NAME="jwtSecretRotationSchedule"
aws events put-rule \
  --name "$RULE_NAME" \
  --schedule-expression "$SCHEDULE"

LAMBDA_ARN=$(aws lambda get-function --function-name "$FUNCTION_NAME" --query 'Configuration.FunctionArn' --output text)

aws events put-targets \
  --rule "$RULE_NAME" \
  --targets "Id=1,Arn=$LAMBDA_ARN"

aws lambda add-permission \
  --function-name "$FUNCTION_NAME" \
  --statement-id "EventBridgeInvoke" \
  --action "lambda:InvokeFunction" \
  --principal events.amazonaws.com \
  --source-arn $(aws events describe-rule --name "$RULE_NAME" --query 'Arn' --output text)

echo "--- Cleaning up ---"
rm main deployment.zip

echo "--- Deployment complete! ---"
`

	gcpScriptTemplate = `#!/bin/bash
# Deployment script for Google Cloud Function

echo "--- Deploying to Google Cloud ---"
# Note: This script assumes you have authenticated with the gcloud CLI and have the necessary permissions.

FUNCTION_NAME="rotateJwtSecret"
SCHEDULE="every 24 hours"
SCHEDULER_JOB_NAME="jwt-rotation-scheduler"

gcloud functions deploy "$FUNCTION_NAME" \
  --runtime go116 \
  --trigger-http \
  --allow-unauthenticated \
  --source deployment/gcp \
  --entry-point RotateSecret \
  --set-env-vars "PROJECT_ID={{.ProjectID}},SECRET_ID={{.SecretID}},SENTRY_DSN={{.SentryDSN}},SLACK_BOT_TOKEN={{.SlackBotToken}},SLACK_CHANNEL_ID={{.SlackChannelID}}"

FUNCTION_URL=$(gcloud functions describe "$FUNCTION_NAME" --format 'value(https_trigger.url)')

echo "--- Creating Cloud Scheduler job ---"
gcloud scheduler jobs create http "$SCHEDULER_JOB_NAME" \
  --schedule="$SCHEDULE" \
  --uri="$FUNCTION_URL" \
  --http-method=GET

echo "--- Deployment complete! ---"
`

	azureScriptTemplate = `#!/bin/bash
# Deployment script for Azure Function

echo "--- Deploying to Azure ---"
# Note: This script assumes you have authenticated with the az CLI and have the Azure Functions Core Tools installed.

RESOURCE_GROUP="{{.ResourceGroupName}}"
STORAGE_ACCOUNT="{{.StorageAccountName}}"
FUNCTION_APP="{{.FunctionAppName}}"
LOCATION="eastus" # Or your preferred location

# Create a resource group
az group create --name "$RESOURCE_GROUP" --location "$LOCATION"

# Create a storage account
az storage account create --name "$STORAGE_ACCOUNT" --location "$LOCATION" --resource-group "$RESOURCE_GROUP" --sku Standard_LRS

# Create a function app
az functionapp create \
  --resource-group "$RESOURCE_GROUP" \
  --storage-account "$STORAGE_ACCOUNT" \
  --consumption-plan-location "$LOCATION" \
  --name "$FUNCTION_APP" \
  --runtime golang

# Set environment variables
az functionapp config appsettings set --name "$FUNCTION_APP" --resource-group "$RESOURCE_GROUP" \
  --settings "VAULT_URI={{.VaultURI}} SECRET_NAME={{.SecretName}} SENTRY_DSN={{.SentryDSN}} SLACK_BOT_TOKEN={{.SlackBotToken}} SLACK_CHANNEL_ID={{.SlackChannelID}}"

# Deploy the function
# Note: This requires the Azure Functions Core Tools (func) to be installed.
# cd into the function directory to deploy
(cd deployment/azure && func azure functionapp publish "$FUNCTION_APP")

echo "--- Deployment complete! ---"
`
)

// GenerateScript generates a deployment script for the given provider.
func GenerateScript(data ScriptData) (string, error) {
	var tpl string
	switch data.Provider {
	case "AWS":
		tpl = awsScriptTemplate
	case "GCP":
		tpl = gcpScriptTemplate
	case "Azure":
		tpl = azureScriptTemplate
	default:
		return "", fmt.Errorf("unknown provider: %s", data.Provider)
	}

	tmpl, err := template.New("script").Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse script template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute script template: %w", err)
	}

	return buf.String(), nil
}
