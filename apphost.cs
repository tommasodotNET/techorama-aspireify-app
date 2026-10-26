#:package Aspire.Hosting.Azure.CosmosDB@13.6.4
#:package Aspire.Hosting.Docker@13.6.4
#:package Aspire.Hosting.Go@13.6.4-preview.1.26523.11
#:package Aspire.Hosting.JavaScript@13.6.4
#:sdk Aspire.AppHost.Sdk@13.6.4
#:property AspireUseCliBundle=true

using System.Net.Http.Headers;
using System.Text;
using Aspire.Hosting.ApplicationModel;

var builder = DistributedApplication.CreateBuilder(args);

builder.AddDockerComposeEnvironment("compose")
    .ConfigureComposeFile(file =>
    {
        file.Services["compose-dashboard"].Ports.Clear();
        file.Services["compose-dashboard"].Ports.Add("127.0.0.1:18888:18888");
        file.Services["frontend"].DependsOn["api"] = new() { Condition = "service_healthy" };
    });

if (builder.ExecutionContext.IsRunMode &&
    builder.Configuration["Parameters:admin-password"] is null &&
    builder.Configuration["Parameters:admin_password"] is null)
{
    builder.Configuration["Parameters:admin-password"] = "alligator123";
}
var adminPassword = builder.AddParameter("admin-password", secret: true);

var cosmos = builder.AddAzureCosmosDB("cosmos")
    .WithAccessKeyAuthentication()
    .RunAsEmulator(emulator => emulator.WithDataVolume("purr-and-prose-aspire-cosmos-data"));
// var cosmos = builder.AddAzureCosmosDB("cosmos").WithAccessKeyAuthentication();
var books = cosmos.AddCosmosDatabase("catbooks").AddContainer("books", "/shelf");

var backend = builder.AddGoApp("api", "./src/api")
    .WithHttpEndpoint(env: "PORT")
    .WithHttpHealthCheck("/healthz")
    .WithEnvironment("ADMIN_PASSWORD", adminPassword)
    .WithReference(books)
    .WaitFor(books)
    .WithHttpCommand("/api/admin/database/reset", "Clear and seed database",
        endpointName: "http",
        commandName: "reset-database",
        commandOptions: new HttpCommandOptions
        {
            Description = "Delete every item in the configured books container and restore the four sample books.",
            ConfirmationMessage = "This permanently deletes all listings in the configured books container. Clear it and restore the four sample books?",
            IsHighlighted = true,
            ResultMode = HttpCommandResultMode.Json,
            PrepareRequest = async context =>
            {
                var password = await adminPassword.Resource.GetValueAsync(context.CancellationToken);
                context.Request.Headers.Authorization = new AuthenticationHeaderValue(
                    "Basic", Convert.ToBase64String(Encoding.UTF8.GetBytes($"admin:{password}")));
                context.Request.Headers.Add("X-Confirm-Reset", "clear-and-seed");
            }
        })
    .PublishAsDockerFile(container => container
        .WithDockerfile("./src/api")
        .WithEndpoint("http", endpoint => endpoint.TargetPort = 8080))
    .PublishAsDockerComposeService((_, service) =>
    {
        service.Ports.Clear();
        service.Ports.Add("127.0.0.1:8086:8080");
        service.Environment["ADMIN_PASSWORD"] = "${ADMIN_PASSWORD:?Set ADMIN_PASSWORD before starting this deployment}";
        service.Restart = "unless-stopped";
        service.Healthcheck = new()
        {
            Test = ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost:8080/healthz"],
            Interval = "5s", Timeout = "4s", StartPeriod = "10s", Retries = 20
        };
    });

builder.AddViteApp("frontend", "./src/frontend")
    .WithReference(backend)
    .WithHttpHealthCheck("/healthz")
    .WaitFor(backend)
    .WithExternalHttpEndpoints()
    .PublishAsDockerFile(container => container
        .WithEndpoint("http", endpoint => { endpoint.TargetPort = 80; endpoint.Port = 8088; }))
    .PublishAsDockerComposeService((_, service) =>
    {
        service.Ports.Clear();
        service.Ports.Add("127.0.0.1:8088:80");
        service.Restart = "unless-stopped";
        service.Healthcheck = new()
        {
            Test = ["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1/healthz"],
            Interval = "10s", Timeout = "4s", StartPeriod = "5s", Retries = 5
        };
    });

builder.Build().Run();