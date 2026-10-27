FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /source
COPY Store.Api.csproj .
RUN dotnet restore
COPY . .
RUN dotnet publish --no-restore -c Release -o /app

FROM mcr.microsoft.com/dotnet/aspnet:10.0
USER root
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /app .
USER $APP_UID
EXPOSE 8080
ENTRYPOINT ["dotnet", "Store.Api.dll"]
