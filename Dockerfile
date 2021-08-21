FROM golang:latest
RUN apt-get update
RUN apt-get -y install ffmpeg
RUN mkdir /app
WORKDIR /app
VOLUME /app

# Expose port 9090 to the outside world
EXPOSE 9090