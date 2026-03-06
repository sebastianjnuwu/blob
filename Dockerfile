FROM node:20-alpine

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

COPY . .

# Ensure local data volume path exists for sqlite and object files.
RUN mkdir -p /app/data/blob-storage

ENV NODE_ENV=production
EXPOSE 3000

CMD ["sh", "-c", "npm run db:prepare && npm start"]
