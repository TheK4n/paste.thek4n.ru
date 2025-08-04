<h1 align="center">Copy/Paste and URL shortener web service</h1>

<p align="center">
  <a href="https://github.com/TheK4n">
    <img src="https://img.shields.io/github/followers/TheK4n?label=Follow&style=social">
  </a>
  <a href="https://github.com/TheK4n/paste.thek4n.ru">
    <img src="https://img.shields.io/github/stars/TheK4n/paste.thek4n.ru?style=social">
  </a>
</p>

* [Setup](#setup)
* [Usage](#usage)

---

Copy/Paste and URL shortener web service


## Setup
```sh
cd "$(mktemp -d)"
git clone https://github.com/thek4n/paste.thek4n.ru .
docker compose up -d
```


## Usage

### API
Put text and get it by unique url
```sh
URL="$(curl -d 'Hello' 'localhost:8081/')"
echo "${URL}"  # http://localhost:8081/8fYfLk34Y1H3UQ/
curl "${URL}"  # Hello
```

---

Put text with expiration time
```sh
curl -d 'Hello' 'localhost:8081/?ttl=3h'
curl -d 'Hello' 'localhost:8081/?ttl=30m'
URL="$(curl -d 'Hello' 'localhost:8081/?ttl=60s')"

sleep 61 && curl -i "${URL}"  # 404 Not Found
```

Put persist url (allowed only for authorized apikeys)
```sh
curl -d 'https://example.com/' 'localhost:8081/?url=true&ttl=0&apikey=apikey'
```

Put disposable text
```sh
URL="$(curl -d 'Hello' 'localhost:8081/?disposable=1')"
curl -i "${URL}"  # Hello
curl -i "${URL}"  # 404 Not Found
```

```sh
URL="$(curl -d 'Hello' 'localhost:8081/?disposable=2')"
curl -i "${URL}"  # Hello
curl -i "${URL}"  # Hello
curl -i "${URL}"  # 404 Not Found
```

Put URL to redirect
```sh
URL="$(curl -d 'https://example.com/' 'localhost:8081/?url=true')"
curl -iL "${URL}"  # 303 See Other
```

Get clicks
```sh
curl -iL "${URL}/clicks/"  # 1
```

Put disposable url with 3 minute expiration time
```sh
URL="$(curl -d 'https://example.com/' 'localhost:8081/?url=true&disposable=1&ttl=3m')"
curl -iL "${URL}"  # 303 See Other
curl -iL "${URL}"  # 404 Not found
```


Put text with custom key length in range from 14 to 20
```sh
curl -d 'https://example.com/' 'localhost:8081/?url=true&len=20'
# http://localhost:8081/8fYfLk34Y1H3UQ213as1/
```

Range from 3 to 13 allowed only for authorized apikeys
```sh
curl -d 'https://example.com/' 'localhost:8081/?url=true&len=3&apikey=apikey'
# http://localhost:8081/Dav/
```

Authorized apikeys can request custom key
```sh
curl -d 'https://example.com/' 'localhost:8081/?url=true&key=hello&apikey=apikey'
# http://localhost:8081/hello/
```

---

Non authorized has quota 50 post requests in 24 hours


### APIKEYS
Generate new api key:
```sh
export REDIS_HOST=localhost  # Host of redis db or container
./tools/apikeys gen                # generate and add new api key
./tools/apikeys list               # list of api keys
./tools/apikeys revoke "key"       # revoke (invalidate) api key
./tools/apikeys reauthorize "key"  # reauthorize api key
./tools/apikeys rm "key"           # remove api key
```


## Building
```sh
make
```
Output in directory `bin`

### Building with embedded frontend
```sh
VITE_API_URL=http://localhost:8080 make build-frontend
```


## Tests
Before test you need to setup redis-db and rabbitmq. In docker for example:
```sh
docker run --rm -d -p 6379 --name redis redis && \
docker run --rm -d -p 5672:5672 -p 15672:15672 --name rabbitmq rabbitmq:3-management
```
Run tests:
```sh
make test
```

<h1 align="center"><a href="#top">▲</a></h1>
