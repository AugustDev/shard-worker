FROM golang:1.22-bullseye

RUN apt-get update && apt-get install -y \
    openjdk-11-jre-headless \
    curl \
    bash \
    unzip \
    && apt-get clean

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

# Install AWS CLI
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install \
    && rm -rf aws awscliv2.zip

CMD ["./main"]