package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/corehandlers"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GcpTokenProvider implements credentials.Provider interface
type GcpTokenProvider struct {
	token *oauth2.Token
}

var (
	bucket    string
	projectId string
)

func (p *GcpTokenProvider) Retrieve() (credentials.Value, error) {
	ctx := context.Background()

	tokenSource, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return credentials.Value{}, err
	}

	token, err := tokenSource.Token()
	if err != nil {
		return credentials.Value{}, err
	}
	p.token = token

	return credentials.Value{
		AccessKeyID:     token.AccessToken,
		SecretAccessKey: "",
		SessionToken:    "",
		ProviderName:    "",
	}, nil
}

func (p *GcpTokenProvider) IsExpired() bool {
	if p.token == nil {
		return true
	}

	// token has no expiration
	if p.token.Expiry.IsZero() {
		return false
	}

	if p.token.Expiry.Before(time.Now()) {
		return true
	}

	return false
}

func main() {
	flag.StringVar(&bucket, "bucket", "", "bucket name to upload file to")
	flag.StringVar(&projectId, "projectId", "", "projectId to list buckets")
	flag.Parse()

	if bucket == "" || projectId == "" {
		flag.Usage()
		os.Exit(1)
	}

	cred := credentials.NewCredentials(&GcpTokenProvider{})

	config := &aws.Config{
		Credentials: cred,
		Endpoint:    aws.String("https://storage.googleapis.com"),
		Region:      aws.String("us-east-1"), // dummy
	}
	config = config.WithLogLevel(aws.LogDebug)

	ses, err := session.NewSession(config)
	if err != nil {
		log.Fatalf("Failed to create a session: %s", err)
	}
	svc := s3.New(ses)

	// modify HTTP request
	svc.Handlers.Sign.Clear()
	svc.Handlers.Sign.PushBackNamed(corehandlers.BuildContentLengthHandler)
	svc.Handlers.Sign.PushBackNamed(request.NamedHandler{Name: "gcp-oauth2-signer", Fn: func(req *request.Request) {
		credValue, err := config.Credentials.Get()
		if err != nil {
			log.Fatalf("Failed to get a credential: %s", err)
		}
		// add authorization header
		req.HTTPRequest.Header.Set("Authorization", "Bearer "+credValue.AccessKeyID)
		req.HTTPRequest.Header.Set("x-goog-project-id", projectId)
	}})
	svc.Handlers.Send.PushFrontNamed(request.NamedHandler{Name: "aws-to-gcp-headers", Fn: func(req *request.Request) {
		// add "x-goog-*" headers corresponding to "x-amz-*" headers
		for key := range req.HTTPRequest.Header {
			lowerKey := strings.ToLower(key)
			if strings.HasPrefix(lowerKey, "x-amz-") {
				newKey := strings.Replace(lowerKey, "x-amz-", "x-goog-", 1)
				req.HTTPRequest.Header.Set(newKey, req.HTTPRequest.Header.Get(key))
			}
		}

		// remove "x-amz-*" headers
		for key := range req.HTTPRequest.Header {
			if strings.HasPrefix(strings.ToLower(key), "x-amz-") {
				req.HTTPRequest.Header.Del(key)
			}
		}
	}})

	// modify HTTP response
	svc.Handlers.Send.PushBackNamed(request.NamedHandler{Name: "gcp-to-aws-headers", Fn: func(req *request.Request) {
		// add "x-amz-*" headers corresponding to "x-goog-*" headers
		for key := range req.HTTPResponse.Header {
			lowerKey := strings.ToLower(key)
			if strings.HasPrefix(lowerKey, "x-goog-") {
				newKey := strings.Replace(lowerKey, "x-goog-", "x-amz-", 1)
				req.HTTPResponse.Header.Set(newKey, req.HTTPResponse.Header.Get(key))
			}
		}

		// remove "x-goog-*" headers
		for key := range req.HTTPResponse.Header {
			if strings.HasPrefix(strings.ToLower(key), "x-goog-") {
				req.HTTPResponse.Header.Del(key)
			}
		}
	}})

	// List buckets
	result, err := svc.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		fmt.Errorf(err.Error())
	}

	fmt.Println(result)

	putObj, err := svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("/filename.txt"),
		ACL:    aws.String("private"),
		Body:   strings.NewReader("lorem ipsum"),
		Metadata: map[string]*string{
			"key01": aws.String("foo"),
			"key02": aws.String("bar"),
		},
	})
	if err != nil {
		log.Fatalf("Failed to put object: %s", err)
	}
	fmt.Printf("put object: %s\n", putObj)

	getObj, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("/test"),
	})
	fmt.Printf("get object: %#v\n", getObj)
}
