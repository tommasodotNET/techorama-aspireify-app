#:package Aspire.Hosting.Azure.AppContainers@13.6.4
#:package Aspire.Hosting.Azure.Provisioning.AppContainers@13.6.4-preview.1.26523.11
#:package Aspire.Hosting.JavaScript@13.6.4
#:package Aspire.Hosting.PostgreSQL@13.6.4
#:sdk Aspire.AppHost.Sdk@13.6.4
#:property AspireUseCliBundle=true
#:project ./src/api/Store.Api.csproj

var builder = DistributedApplication.CreateBuilder(args);

var aca = builder.AddAzureContainerAppEnvironment("aca");

var adminPassword = builder.AddParameter("websiteAdmin");

var pg = builder.AddPostgres("db");
var db = pg.AddDatabase("store");

var api = builder.AddProject<Projects.Store_Api>("api")
    .WithEnvironment("Admin__Password", adminPassword)
    .WithReference(db)
    .WaitFor(db);

#pragma warning disable 
builder.AddViteApp("frontend", "./src/frontend")
    .WithEnvironment("API_UPSTREAM", api.GetEndpoint("http"))
    .PublishAsStaticWebsite();

builder.Build().Run();