FROM golang:latest
LABEL maintainer "Kellerman81 <Kellerman81@gmail.com>"
RUN apt-get update
RUN ln -fs /usr/share/zoneinfo/Europe/Berlin /etc/localtime
RUN apt-get -y install ffmpeg graphviz gv tzdata
RUN dpkg-reconfigure --frontend noninteractive tzdata
RUN mkdir /app
RUN mkdir /app/config
RUN mkdir /app/databases
RUN mkdir /app/logs
RUN mkdir /app/backup
RUN mkdir /app/temp
WORKDIR /app
USER 0:0
VOLUME /app/databases
VOLUME /app/config
VOLUME /app/logs
COPY ./config/ /app/config/
COPY ./databases/ /app/databases/
COPY ./docs/ /app/docs/
COPY ./schema/ /app/schema/
COPY ./LICENSE /app/LICENSE
COPY ./README.md /app/README.md
COPY ./go_media_downloader* /app/
COPY ./init_imdb* /app/

# Expose port 9090 to the outside world
EXPOSE 9090
CMD ["/app/go_media_downloader"]