watch:
	npx nodemon -e go,html -x "make run || exit 1" --signal SIGTERM

run:
	go run main.go sample.jsonl
