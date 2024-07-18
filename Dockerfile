FROM golang:1.22-bullseye

RUN apt-get update
RUN apt-get install -y openjdk-11-jre-headless 
RUN apt-get install -y curl bash 
RUN apt-get clean

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o main ./cmd

# Install Nextflow
RUN curl -s https://get.nextflow.io | bash
RUN mv nextflow /usr/local/bin/

# Install float
RUN curl -k -L https://44.207.4.113/float -o /usr/local/bin/float && chmod +x /usr/local/bin/float

CMD ["./main"]