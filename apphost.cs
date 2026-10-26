#:package Aspire.Hosting.Azure.CosmosDB@13.6.4
#:package Aspire.Hosting.Go@13.6.4-preview.1.26523.11
#:package Aspire.Hosting.JavaScript@13.6.4
#:sdk Aspire.AppHost.Sdk@13.6.4
#:property AspireUseCliBundle=true

var builder = DistributedApplication.CreateBuilder(args);

var cosmos = builder.AddAzureCosmosDB("cosmos")
    .RunAsEmulator(emulator => emulator.WithDataVolume("purr-and-prose-aspire-cosmos-data"));
var db = cosmos.AddCosmosDatabase("catbooks");
var books = db.AddContainer("books", "/shelf");

var backend = builder.AddGoApp("api", "./src/api")
    .WithHttpEndpoint()
    .WithHttpHealthCheck("/healthz")
    .WithReference(books)
    .WaitFor(books)
    .WithEnvironment("COSMOS_KEY", cosmos.Resource.AccountKey)
    .WithEnvironment("COSMOS_DATABASE", db.Resource.DatabaseName)
    .WithEnvironment("COSMOS_CONTAINER", books.Resource.ContainerName)
    .WithEnvironment("ADMIN_PASSWORD", "alligator123")
    .WithEnvironment("COSMOS_ENDPOINT", cosmos.GetEndpoint("emulator"));

backend.WithEnvironment("HTTP_ADDR", $":{backend.GetEndpoint("http").Property(EndpointProperty.TargetPort)}");

builder.AddViteApp("frontend", "./src/frontend", "dev")
    .WithEnvironment("API_UPSTREAM", backend.GetEndpoint("http"))
    .WithHttpHealthCheck("/healthz")
    .WaitFor(backend);

builder.Build().Run();