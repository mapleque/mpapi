upstream mpapi_server {
    server mpapi.domain;
}

upstream app_server {
    server app.domain;
}

server {
    listen 80;
    location / {
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_pass http://mpapi_server;
    }

    location /app/ {
        rewrite ^\/app\/(.*)$ /u_proxy break;
        proxy_set_header User-Proxy-To http://app.domain/$1;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_pass http://mpapi_server;
    }
}

