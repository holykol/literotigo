# Literotigo
[![works badge](https://cdn.jsdelivr.net/gh/nikku/works-on-my-machine@v0.2.0/badge.svg)](https://github.com/nikku/works-on-my-machine)
[![coverage report](https://gitlab.com/holykol/literotigo/badges/master/coverage.svg)](https://gitlab.com/holykol/literotigo/-/commits/master)
[![pipeline status](https://gitlab.com/holykol/literotigo/badges/master/pipeline.svg)](https://gitlab.com/holykol/literotigo/-/commits/master)

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