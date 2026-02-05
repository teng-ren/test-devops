FROM node:22-alpine

WORKDIR /app

COPY . .

RUN npm install

ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL

RUN npm run build

EXPOSE 5173

CMD ["sh", "-c", "npm run preview -- --port $PORT --host"]

