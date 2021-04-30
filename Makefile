watch:
	npx nodemon -e go,html -x "make run || exit 1" --signal SIGTERM

run:
	go run main.go database.jsonl

test:
	npx nodemon -e go,html -x "go test || exit 1" --signal SIGTERM