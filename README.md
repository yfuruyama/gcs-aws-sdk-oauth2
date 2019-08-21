gcs-aws-sdk-oauth2
===
Go example for accessing GCS using AWS SDK with OAuth2.

This is the example codes for medium article, [Accessing Google Cloud Storage using AWS SDK and OAuth2](https://medium.com/@salmaan.rashid/accessing-google-cloud-storage-using-aws-sdk-and-oauth2-1c7764025810).

## How to run sample codes

```sh
export GO111MODULE=on

go run main.go --bucket=<GCS_BUCKET_NAME> --projectId=<GCP_PROJECT_ID>
```
