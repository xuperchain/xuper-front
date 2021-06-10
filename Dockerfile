FROM ubuntu:20.10
RUN apt-get update && apt-get install -y tzdata && rm -f /etc/localtime && ln -s /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && dpkg-reconfigure -f noninteractive tzdata && apt-get clean && mkdir /work /work/conf /work/data /work/data/db

WORKDIR /work/
COPY ./output/xfront /work/
CMD ["./xfront"]
 
EXPOSE 8084 8085