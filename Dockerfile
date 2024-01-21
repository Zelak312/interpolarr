FROM nvidia/cuda:12.3.1-devel-ubuntu22.04 as builder

# Install dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    yasm \
    nasm \
    pkg-config \
    libva-dev \
    libvdpau-dev \
    libdrm-dev \
    libx264-dev \
    libx265-dev \
    libvpx-dev \
    libfdk-aac-dev \
    libmp3lame-dev \
    libopus-dev \
    libpng-dev \
    wget \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Clone and install the NV Codec Headers
RUN git clone https://git.videolan.org/git/ffmpeg/nv-codec-headers.git \
    && cd nv-codec-headers \
    && make \
    && make install

# Clone the FFMPEG source code
RUN git clone https://git.ffmpeg.org/ffmpeg.git ffmpeg

# Compile FFMPEG from source
WORKDIR /ffmpeg
RUN ./configure \
    --enable-gpl \
    --enable-nonfree \
    --enable-cuda-nvcc \
    --enable-libnpp \
    --enable-nvenc \
    --enable-nvdec \
    --enable-vaapi \
    --enable-libx264 \
    --enable-libx265 \
    --enable-encoders \
    --extra-cflags=-I/usr/local/cuda/include \
    --extra-ldflags=-L/usr/local/cuda/lib64 \
    && make -j$(nproc) \
    && make install

######################################################

FROM golang:1.21-alpine AS golang-base

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod .
COPY go.sum .

# Copy all .go files in the current directory
COPY *.go ./

# Copy the entire migrations directory
COPY migrations ./migrations/

RUN go mod download && go build -o interpolarr .

######################################################

FROM nvidia/cuda:12.3.1-runtime-ubuntu22.04

# Install FFMPEG runtime dependencies
RUN apt-get update && apt-get install -y \
    libva-dev \
    libvdpau1 \
    libdrm2 \
    libx264-dev \
    libx265-dev \
    libvpx-dev \
    libfdk-aac-dev \
    libmp3lame-dev \
    libopus-dev \
    libpng-dev \
    libvulkan1 \
    libgomp1 \
    wget \
    unzip \
    gosu \
    && rm -rf /var/lib/apt/lists/*

# Copy ffmpeg build and interpolarr
WORKDIR /interpolarr
COPY --from=builder /usr/local /usr/local
COPY --from=golang-base /app/interpolarr /interpolarr/interpolarr
COPY entrypoint.sh ./
COPY .env.docker ./
COPY docker_default.yml ./config.yml

# Script to fetch the latest release of RIFE NCNN Vulkan
RUN wget https://api.github.com/repos/TNTwise/rife-ncnn-vulkan/releases/latest \
    -O - | grep "browser_download_url.*ubuntu.zip" | cut -d : -f 2,3 | tr -d \" | wget -qi - \
    && mkdir temp_dir && unzip ubuntu.zip -d temp_dir \
    && mv temp_dir/ubuntu/* ./ \
    && rm -r temp_dir \
    && rm ubuntu.zip \
    && chmod +x ./entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]