build:
	go build -o lambda_handler -v main.go

zip: build
	zip handler.zip lambda_handler

deploy: zip  
	aws lambda update-function-code \
	  --region us-east-1 \
      --function-name  EKS-Create-ConfigMap \
      --zip-file fileb://${PWD}/handler.zip \

clean: 
	rm -f lambda_handler handler.zip
