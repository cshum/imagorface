build:
	go build -o bin/imagorface ./cmd/imagorface/main.go

test:
	go clean -testcache && go test -coverprofile=profile.cov ./...

dev: build
	./bin/imagorface -debug -imagor-unsafe -face-detector

help: build
	./bin/imagorface -h

get:
	go get -v -t -d ./...

docker-dev-build:
	docker build -t imagorface:dev .

docker-dev-run:
	touch .env
	docker run --rm -p 8000:8000 --env-file .env imagorface:dev -debug -imagor-unsafe -face-detector

docker-dev: docker-dev-build docker-dev-run
