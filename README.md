<h1 align="center">Copy/Paste web service</h1>

<p align="center">
  <a href="https://github.com/TheK4n">
    <img src="https://img.shields.io/github/followers/TheK4n?label=Follow&style=social">
  </a>
  <a href="https://github.com/TheK4n/paste.thek4n.name">
    <img src="https://img.shields.io/github/stars/TheK4n/paste.thek4n.name?style=social">
  </a>
</p>

* [Setup](#setup)
* [Usage](#usage)

---

Copy/Paste web service


## Setup

```sh
cd "$(mktemp -d)"
git clone https://github.com/thek4n/paste.thek4n.name .
docker compose up -d
```


## Usage

Put text and get it by unique url
```sh
echo "hello" | curl --data-binary @- localhost:8080/
# http://localhost:8080/8fYfLk34Y1H3UQ/

curl http://localhost:8080/8fYfLk34Y1H3UQ/
# hello
```

---

Put text with expiration time
```sh
echo "hello" | curl --data-binary @- 'localhost:8080/?ttl=3h'
echo "hello" | curl --data-binary @- 'localhost:8080/?ttl=30m'
echo "hello" | curl --data-binary @- 'localhost:8080/?ttl=60s'
```

Put disposable text
```sh
echo "hello" | curl --data-binary @- 'localhost:8080/?disposable=true'
url -i http://localhost:8080/V6A6NySdsnGuFS/  ## 200 OK
curl -i http://localhost:8080/V6A6NySdsnGuFS/  ## 404 Not found
```


<h1 align="center"><a href="#top">▲</a></h1>
