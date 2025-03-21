FROM ubuntu:24.04 AS builder

RUN apt update -y && apt install -y npm

WORKDIR /build

COPY . .

RUN npm install && npm run build


FROM nginx

WORKDIR /usr/share/nginx/html

COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /build/dist/ .

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
