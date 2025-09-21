:: proxy
set HTTP_PROXY=http://127.0.0.1:1080
set HTTPS_PROXY=http://127.0.0.1:1080

:: build docker image
docker build ./ -t lianshufeng/proxy-pool