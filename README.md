# Literotigo
Simple frontend for some site dump I found online ( ͡° ͜ʖ ͡°)

### Features
* In-memory index of authors and caterories
* Parallel indexing (bottlenecked by disk speed anyway)
* Plain HTML web interface using html/template
* Zero dependencies (except that ~20 loc mmap package)

### Running
```sh
# Download database. This could take a while
curl https://the-eye.eu/public/AI/pile_preliminary_components/Literotica.jsonl.zst | zstdcat > database.jsonl

# Run directly
make run

# Or use Docker
docker build -t literotigo .
docker run -v $(pwd)/database.jsonl:/var/database.jsonl -p "8080:8080" literotigo
```

### License
Releases under WTFPL