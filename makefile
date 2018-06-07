build:
	go build -o lambda_handler -v main.go

zip: build
	zip handler.zip lambda_handler.go

deploy: zip  
	aws lambda create-function \
	  --region us-east-1 \
      --function-name lambda-handler \
      --memory 128 \
      --role arn:aws:iam::account-id:role/execution_role \
      --runtime go1.x \
      --zip-file fileb://${PWD}/handler.zip \
      --handler lambda-handler
