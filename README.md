# Go Secrets Rotator

Flexible tool written in Go for automating the rotation of secrets. It is designed to be a hybrid-cloud solution that helps you improve your security posture by regularly rotating sensitive credentials.

While the core rotation engine is generic, the tool is currently focused on rotating **JWT Signing Secrets**.

---

## Core Features

- **Interactive Terminal UI:** A polished and user-friendly command-line interface (built with Bubble Tea) guides you through the entire configuration process.
- **Multi-Cloud Support:** Store and manage your secrets in any of the three major cloud providers:
  - **AWS Secrets Manager**
  - **Google Cloud Secret Manager**
  - **Azure Key Vault**
- **Pluggable Observability:** Get notifications about rotation events in your favorite monitoring tools.
  - **Sentry**
  - **Slack**
  - *(Support for both at the same time is available)*
- **Flexible Rotation Modes:**
  - **Run Once:** Perform an immediate, one-time rotation of your secret.
  - **Periodic Rotation (Serverless Deployment):** The tool generates the necessary code and provides simple, copy-paste instructions to deploy a serverless function (AWS Lambda, Google Cloud Function, or Azure Function) to your cloud provider. This provides a robust, hands-off solution for automated rotation.

---

## How to Use

To get started, run the interactive CLI:

```bash
go run main.go
```

The tool will guide you through the following steps:

1.  **Choose Your Cloud Provider:** Select AWS, GCP, or Azure.
2.  **Enter Configuration:** Provide the necessary configuration for your chosen provider (e.g., GCP Project ID and Secret ID).
3.  **Select Notification Channels:** Choose whether you want to receive notifications in Sentry, Slack, both, or neither.
4.  **Select Rotation Mode:** Choose whether you want to run the rotation once or set up a periodic rotation via a serverless function.
5.  **Execute or Get Instructions:**
    - If you chose **"Run once,"** the tool will perform the rotation and then exit.
    - If you chose **"Run periodically,"** the tool will display a detailed set of instructions for deploying the serverless function to your cloud provider.

---

## Serverless Deployment Configuration

To run the deployed serverless function, you will need to configure some environment variables.

### Cloud Provider Credentials

The serverless function will need credentials to access your secret manager. This is typically handled by assigning an **IAM Role** (or equivalent) to the function with the necessary permissions.

You will also need to set the following provider-specific environment variables:

-   **AWS:** `SECRET_ID`, `REGION`
-   **GCP:** `PROJECT_ID`, `SECRET_ID`
-   **Azure:** `VAULT_URI`, `SECRET_NAME`

### Notifier Configuration

To enable notifications, set the following environment variables:

-   **Sentry:**
    -   `SENTRY_DSN`: Your Sentry DSN.
-   **Slack:**
    -   `SLACK_BOT_TOKEN`: Your Slack Bot Token (starts with `xoxb-`).
    -   `SLACK_CHANNEL_ID`: The ID of the channel to post in.

---

## Architecture Overview

The tool is built on a modular and extensible architecture:

-   **`RotationManager`:** A generic secret rotation engine that handles the core rotation logic.
-   **`JWTManager`:** A specialized manager built on top of the `RotationManager` to handle JWT-specific operations like signing and validating tokens.
-   **`SecretStorage` Interface:** A pluggable storage interface that allows the tool to support different cloud backends.
-   **`Notifier` Interface:** A pluggable notification interface that makes it easy to add new observability tools.

This design makes the tool easy to maintain and extend with new secret types, storage backends, or notifiers in the future.

---

## Implementation Details

### JWT Secret Generation

When a new JWT signing secret is generated, the tool performs the following steps to ensure a high level of security:

1.  **Cryptographically Secure Randomness:** The secret is created by generating a 64-byte slice filled with cryptographically secure random data using Go's standard `crypto/rand` library. This ensures that the generated secrets are unpredictable and suitable for cryptographic operations.

2.  **Unique Secret ID:** A unique ID (`kid`) is generated for each secret. This is done by creating an HMAC-SHA256 hash of the secret value itself. The first 12 characters of the resulting hex-encoded hash are used as the secret's ID. This ID is then embedded in the header of any JWTs signed with this secret, allowing for seamless validation during the key rotation grace period.
