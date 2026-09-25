# Start from a lightweight Rust base image
FROM rust:1.76-slim

# Install curl, then use it to install Node.js (v20)
RUN apt-get update && apt-get install -y curl && \
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Add the WebAssembly target required for compiling Soroban contracts
RUN rustup target add wasm32-unknown-unknown

# Set up the application directory
WORKDIR /app

# Install Node dependencies
COPY package*.json ./
RUN npm install --production

# Copy the server code
COPY server.js ./

# Render assigns a PORT dynamically, but we expose 8080 as a fallback
ENV PORT=8080
EXPOSE 8080

# Start the Express API
CMD ["node", "server.js"]
