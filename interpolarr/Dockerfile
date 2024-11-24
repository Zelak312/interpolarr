FROM nvidia/cuda:12.3.1-devel-ubuntu22.04 as builder

# Install dependencies for NV Codec headers and FFmpeg
RUN apt-get update && apt-get install -y \
    # General build tools (used by both NV Codec headers and FFmpeg)
    build-essential \
    git \
    pkg-config \
    wget \
    unzip \
    # Assembling and linking tools (used by FFmpeg)
    yasm \
    nasm \
    # Libraries required for NV Codec headers (NVIDIA video acceleration)
    libva-dev \
    libvdpau-dev \
    libdrm-dev \
    # Video and audio codec libraries (used by FFmpeg)
    libx264-dev \
    libx265-dev \
    libvpx-dev \
    libfdk-aac-dev \
    libmp3lame-dev \
    libopus-dev \
    libpng-dev \
    # Cleanup package manager cache to reduce image size
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

FROM golang:1.23.0-bullseye AS golang-base

# RUN apt-get update && apt-get install -y \
#     git \

WORKDIR /app
ENTRYPOINT ["/bin/sh"]
# # Copy go.mod and go.sum files
# COPY go.mod .
# COPY go.sum .

# # Copy all .go files in the current directory
# COPY *.go ./

# # Copy the entire migrations directory
# COPY migrations ./migrations/

# # Copy the entire views directory
# COPY views ./views/

# RUN go mod download && go build -tags=release -o interpolarr .

######################################################

# FROM nvidia/cuda:12.3.1-runtime-ubuntu22.04

# # Install FFmpeg runtime dependencies and Vulkan support
# RUN apt-get update && apt-get install -y \
#     # FFmpeg runtime dependencies
#     libva-dev \
#     libvdpau1 \        
#     libdrm2 \          
#     libx264-dev \
#     libx265-dev \
#     libvpx-dev \
#     libfdk-aac-dev \   
#     libmp3lame-dev \   
#     libopus-dev \      
#     libpng-dev \       
#     # Vulkan and GPU-related dependencies
#     libvulkan1 \
#     libgomp1 \
#     libglvnd-dev \
#     libgl1 \
#     libglx0 \
#     libegl1 \
#     libgles2 \
#     libglx-mesa0 \
#     # Miscellaneous tools
#     wget \             
#     unzip \
#     gosu \
#     # Cleanup package manager cache to reduce image size
#     && rm -rf /var/lib/apt/lists/*

# # Copy ffmpeg build and interpolarr
# WORKDIR /interpolarr
# COPY --from=builder /usr/local /usr/local
# COPY --from=golang-base /app/interpolarr /interpolarr/interpolarr
# COPY entrypoint.sh ./
# COPY .env.docker ./
# COPY docker_default.yml ./config.yml

# ENV NVIDIA_DRIVER_CAPABILITIES=all
# ENV NVIDIA_VISIBLE_DEVICES=all
# EXPOSE 8080
# ENTRYPOINT ["./entrypoint.sh"] 