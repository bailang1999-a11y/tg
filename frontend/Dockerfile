FROM node:22-alpine AS build

ARG APP_VERSION=dev
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build
RUN printf '{"version":"%s"}\n' "$APP_VERSION" > /app/dist/version.json

FROM nginx:1.29-alpine

COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
