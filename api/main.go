package main

import (
	"os"
	"fmt"
	"log"
	"sync"
	"time"
	"bytes"
	"context"
	"reflect"
	"strings"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type APIResponse struct {
	Message  string `json:"message"`
}

type Response events.APIGatewayProxyResponse

var cfg aws.Config
var sqsClient *sqs.Client

const layout  string = "2006-01-02 15:04"
const layout2 string = "20060102150405.000"

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var jsonBytes []byte
	var err error
	d := make(map[string]string)
	json.Unmarshal([]byte(request.Body), &d)
	if v, ok := d["action"]; ok {
		switch v {
		case "sendmessage" :
			if m, ok := d["message"]; ok {
				e := sendMessage(ctx, m)
				if e != nil {
					err = e
				} else {
					jsonBytes, _ = json.Marshal(APIResponse{Message: "Success. Please Receive."})
				}
			}
		case "getcount" :
			message, e := getCount(ctx)
			if e != nil {
				err = e
			} else {
				jsonBytes, _ = json.Marshal(APIResponse{Message: message})
			}
		case "receivemessage" :
			message, e := receiveMessage(ctx)
			if e != nil {
				err = e
			} else {
				jsonBytes, _ = json.Marshal(APIResponse{Message: message})
			}
		}
	}
	log.Print(request.RequestContext.Identity.SourceIP)
	if err != nil {
		log.Print(err)
		jsonBytes, _ = json.Marshal(APIResponse{Message: fmt.Sprint(err)})
		return Response{
			StatusCode: 500,
			Body: string(jsonBytes),
		}, nil
	}
	return Response {
		StatusCode: 200,
		Body: string(jsonBytes),
	}, nil
}

func sendMessage(ctx context.Context, message string) error {
	t := time.Now()
	if sqsClient == nil {
		sqsClient = getSqsClient()
	}
	params := &sqs.SendMessageInput{
		MessageBody:            aws.String(message),
		QueueUrl:               aws.String(os.Getenv("QUEUE_URL")),
		MessageGroupId:         aws.String(os.Getenv("MESSAGE_GROUP_ID")),
		MessageDeduplicationId: aws.String(t.Format(layout2)),
	}
	_, err := sqsClient.SendMessage(ctx, params)
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func getCount(ctx context.Context)(string, error) {
	if sqsClient == nil {
		sqsClient = getSqsClient()
	}
	params := &sqs.GetQueueAttributesInput{
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximatenumberofmessages},
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
	}
	res, err := sqsClient.GetQueueAttributes(ctx, params)
	if err != nil {
		return "", err
	}
	attributeName := string(types.QueueAttributeNameApproximatenumberofmessages)
	return stringValue(res.Attributes[attributeName]), nil
}

func deleteMessage(ctx context.Context, msg types.Message) error {
	if sqsClient == nil {
		sqsClient = getSqsClient()
	}
	params := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(os.Getenv("QUEUE_URL")),
		ReceiptHandle: msg.ReceiptHandle,
	}
	_, err := sqsClient.DeleteMessage(ctx, params)

	if err != nil {
		return err
	}
	return nil
}

func receiveMessage(ctx context.Context)(string, error) {
	if sqsClient == nil {
		sqsClient = getSqsClient()
	}
	params := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
		MaxNumberOfMessages: aws.Int32(1),
		WaitTimeSeconds: aws.Int32(3),
	}
	res, err := sqsClient.ReceiveMessage(ctx, params)
	if err != nil {
		return "", err
	}
	if len(res.Messages) == 0 {
		return "Empty.", nil
	}
	var wg sync.WaitGroup
	for _, m := range res.Messages {
		wg.Add(1)
		go func(msg types.Message) {
			defer wg.Done()
			if err := deleteMessage(ctx, msg); err != nil {
				log.Println(err)
			}
		}(*m)
	}
	wg.Wait()
	return stringValue(res.Messages[0].Body), nil
}

func getSqsClient() *sqs.Client {
	if cfg.Region != os.Getenv("REGION") {
		cfg = getConfig()
	}
	return sqs.NewFromConfig(cfg)
}

func getConfig() aws.Config {
	var err error
	newConfig, err := config.LoadDefaultConfig()
	newConfig.Region = os.Getenv("REGION")
	if err != nil {
		log.Print(err)
	}
	return newConfig
}

func stringValue(i interface{}) string {
	var buf bytes.Buffer
	strVal(reflect.ValueOf(i), 0, &buf)
	return buf.String()
}

func strVal(v reflect.Value, indent int, buf *bytes.Buffer) {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		buf.WriteString("{\n")
		for i := 0; i < v.Type().NumField(); i++ {
			ft := v.Type().Field(i)
			fv := v.Field(i)
			if ft.Name[0:1] == strings.ToLower(ft.Name[0:1]) {
				continue // ignore unexported fields
			}
			if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Slice) && fv.IsNil() {
				continue // ignore unset fields
			}
			buf.WriteString(strings.Repeat(" ", indent+2))
			buf.WriteString(ft.Name + ": ")
			if tag := ft.Tag.Get("sensitive"); tag == "true" {
				buf.WriteString("<sensitive>")
			} else {
				strVal(fv, indent+2, buf)
			}
			buf.WriteString(",\n")
		}
		buf.WriteString("\n" + strings.Repeat(" ", indent) + "}")
	case reflect.Slice:
		nl, id, id2 := "", "", ""
		if v.Len() > 3 {
			nl, id, id2 = "\n", strings.Repeat(" ", indent), strings.Repeat(" ", indent+2)
		}
		buf.WriteString("[" + nl)
		for i := 0; i < v.Len(); i++ {
			buf.WriteString(id2)
			strVal(v.Index(i), indent+2, buf)
			if i < v.Len()-1 {
				buf.WriteString("," + nl)
			}
		}
		buf.WriteString(nl + id + "]")
	case reflect.Map:
		buf.WriteString("{\n")
		for i, k := range v.MapKeys() {
			buf.WriteString(strings.Repeat(" ", indent+2))
			buf.WriteString(k.String() + ": ")
			strVal(v.MapIndex(k), indent+2, buf)
			if i < v.Len()-1 {
				buf.WriteString(",\n")
			}
		}
		buf.WriteString("\n" + strings.Repeat(" ", indent) + "}")
	default:
		format := "%v"
		switch v.Interface().(type) {
		case string:
			format = "%q"
		}
		fmt.Fprintf(buf, format, v.Interface())
	}
}

func main() {
	lambda.Start(HandleRequest)
}
