package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
	"time"
)

var headers = map[string]string{
	"Access-Control-Allow-Origin":  "*",
	"Access-Control-Allow-Methods": "OPTIONS,POST",
	"Access-Control-Allow-Headers": "Content-Type",
	"Content-Type":                 "application/json",
}

var bucket string

func init() {
	bucket = os.Getenv("S3_BUCKET")
}

func Respond(body map[string]string, status int) (events.APIGatewayV2HTTPResponse, error) {
	marshalledBody, err := json.Marshal(body)

	if err != nil {
		return Respond(map[string]string{"error": err.Error()}, http.StatusInternalServerError)
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    headers,
		Body:       string(marshalledBody),
	}, nil
}

func ReturnErrorToUser(error error, status int) (events.APIGatewayV2HTTPResponse, error) {
	log.Println(error.Error())
	body := map[string]string{"error": error.Error()}

	return Respond(body, status)
}

func GeneratePresignedUrl(key string) (url string, err error) {
	sess, err := session.NewSession()

	if err != nil {
		return url, err
	}

	svc := s3.New(sess)

	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	url, err = req.Presign(15 * time.Minute)

	if err != nil {
		return url, err
	}

	return url, nil
}

type Request struct {
	Extension string `json:"extension"`
}

func ParseRequestBody(req events.APIGatewayV2HTTPRequest) (request *Request, err error) {
	b := []byte(req.Body)

	if req.IsBase64Encoded {
		base64Body, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return request, err
		}
		b = base64Body
	}

	err = json.Unmarshal(b, &request)

	if err != nil {
		return request, err
	}

	if request.Extension == "" {
		return request, errors.New("extension is missing")
	}

	return request, nil
}

func GenerateKey(ext string) string {
	fileName := uuid.NewString()
	return fmt.Sprintf("references/%s.%s", fileName, ext)
}

func Handler(_ context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.HTTP.Method == "OPTIONS" {
		return Respond(map[string]string{"status": "ok"}, http.StatusOK)
	}

	request, err := ParseRequestBody(req)
	if err != nil {
		return ReturnErrorToUser(err, http.StatusBadRequest)
	}

	key := GenerateKey(request.Extension)
	url, err := GeneratePresignedUrl(key)

	if err != nil {
		return ReturnErrorToUser(err, http.StatusBadRequest)
	}

	return Respond(map[string]string{"url": url, "key": key}, http.StatusOK)
}

func main() {
	lambda.Start(Handler)
}
