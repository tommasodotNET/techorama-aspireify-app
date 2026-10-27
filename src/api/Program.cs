using System.Security.Cryptography;
using System.Text;
using Npgsql;
using OpenTelemetry;
using OpenTelemetry.Logs;
using OpenTelemetry.Metrics;
using OpenTelemetry.Resources;
using OpenTelemetry.Trace;

var builder = WebApplication.CreateBuilder(args);
builder.Logging.AddOpenTelemetry(options =>
{
    options.IncludeFormattedMessage = true;
    options.IncludeScopes = true;
});
var telemetry = builder.Services.AddOpenTelemetry()
    .ConfigureResource(resource => resource.AddService(
        builder.Configuration["OTEL_SERVICE_NAME"] ?? "tall-order-api"))
    .WithTracing(tracing => tracing
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddSource("Npgsql"))
    .WithMetrics(metrics => metrics
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddRuntimeInstrumentation()
        .AddMeter("Npgsql"));
if (!string.IsNullOrWhiteSpace(builder.Configuration["OTEL_EXPORTER_OTLP_ENDPOINT"]))
    telemetry.UseOtlpExporter();

var connectionString = builder.Configuration.GetConnectionString("Store")
    ?? throw new InvalidOperationException("ConnectionStrings__Store is required.");
var adminPassword = builder.Configuration["Admin:Password"];
if (string.IsNullOrWhiteSpace(adminPassword))
    throw new InvalidOperationException("Admin__Password is required.");

builder.Services.AddSingleton(NpgsqlDataSource.Create(connectionString));
builder.Services.AddProblemDetails();
var app = builder.Build();
app.UseExceptionHandler();

var source = app.Services.GetRequiredService<NpgsqlDataSource>();
for (var attempt = 1; ; attempt++)
{
    try
    {
        await using var connection = await source.OpenConnectionAsync();
        await using var transaction = await connection.BeginTransactionAsync();
        // Serialize initialization if more than one API instance starts.
        await using var command = new NpgsqlCommand(
            "SELECT pg_advisory_xact_lock(740901); " +
            await File.ReadAllTextAsync(Path.Combine(app.Environment.ContentRootPath, "schema.sql")),
            connection, transaction);
        await command.ExecuteNonQueryAsync();
        await transaction.CommitAsync();
        break;
    }
    catch (NpgsqlException error) when (error.IsTransient && attempt < 15)
    {
        app.Logger.LogWarning(error, "Database not ready; initialization attempt {Attempt}/15", attempt);
        await Task.Delay(TimeSpan.FromSeconds(2));
    }
}

app.MapGet("/health", async (NpgsqlDataSource db, CancellationToken ct) =>
{
    await using var command = db.CreateCommand("SELECT 1 FROM seed_history WHERE version = 1");
    return await command.ExecuteScalarAsync(ct) is not null
        ? Results.Ok(new { status = "ready" })
        : Results.StatusCode(503);
});

app.MapGet("/api/products", async (NpgsqlDataSource db, CancellationToken ct) =>
{
    var products = new List<Product>();
    await using var command = db.CreateCommand(
        "SELECT id, name, description, icon, price, stock FROM products WHERE active ORDER BY name");
    await using var reader = await command.ExecuteReaderAsync(ct);
    while (await reader.ReadAsync(ct))
        products.Add(new(reader.GetGuid(0), reader.GetString(1), reader.GetString(2),
            reader.GetString(3), reader.GetDecimal(4), reader.GetInt32(5)));
    return Results.Ok(products);
});

var admin = app.MapGroup("/api/admin");
admin.AddEndpointFilter(async (context, next) =>
{
    var supplied = context.HttpContext.Request.Headers["X-Admin-Password"].ToString();
    return CryptographicOperations.FixedTimeEquals(
        SHA256.HashData(Encoding.UTF8.GetBytes(supplied)),
        SHA256.HashData(Encoding.UTF8.GetBytes(adminPassword)))
        ? await next(context)
        : Results.Problem("Incorrect demo admin password.", statusCode: 401);
});
admin.MapPost("/unlock", () => Results.NoContent());
admin.MapPost("/products", async (NewProduct input, NpgsqlDataSource db, CancellationToken ct) =>
{
    if (string.IsNullOrWhiteSpace(input.Name) || input.Name.Length > 60 ||
        string.IsNullOrWhiteSpace(input.Description) || input.Description.Length > 180 ||
        input.Icon is not ("banana" or "apple" or "orange" or "melon") ||
        input.Price is <= 0 or > 999999.99m || decimal.Round(input.Price, 2) != input.Price ||
        input.Stock is < 0 or > 10000)
        return Results.Problem("Use a name (1-60 characters), description (1-180), a fruit icon, a positive price with at most 2 decimals, and stock from 0 to 10000.", statusCode: 400);

    var product = new Product(Guid.NewGuid(), input.Name.Trim(), input.Description.Trim(),
        input.Icon, input.Price, input.Stock);
    await using var command = db.CreateCommand(
        "INSERT INTO products (id,name,description,icon,price,stock) VALUES ($1,$2,$3,$4,$5,$6)");
    command.Parameters.AddWithValue(product.Id);
    command.Parameters.AddWithValue(product.Name);
    command.Parameters.AddWithValue(product.Description);
    command.Parameters.AddWithValue(product.Icon);
    command.Parameters.AddWithValue(product.Price);
    command.Parameters.AddWithValue(product.Stock);
    await command.ExecuteNonQueryAsync(ct);
    return Results.Created($"/api/products/{product.Id}", product);
});
admin.MapDelete("/products/{id:guid}", async (Guid id, NpgsqlDataSource db, CancellationToken ct) =>
{
    await using var command = db.CreateCommand("UPDATE products SET active=false WHERE id=$1 AND active");
    command.Parameters.AddWithValue(id);
    return await command.ExecuteNonQueryAsync(ct) == 1
        ? Results.NoContent()
        : Results.Problem("Fruit not found.", statusCode: 404);
});

app.MapPost("/api/orders", async (Purchase input, NpgsqlDataSource db, CancellationToken ct) =>
{
    await using var connection = await db.OpenConnectionAsync(ct);
    await using var transaction = await connection.BeginTransactionAsync(ct);
    await using var command = new NpgsqlCommand(
        "UPDATE products SET stock=stock-1 WHERE id=$1 AND active AND stock>0 RETURNING name, price",
        connection, transaction);
    command.Parameters.AddWithValue(input.ProductId);
    string name;
    decimal price;
    await using (var reader = await command.ExecuteReaderAsync(ct))
    {
        if (!await reader.ReadAsync(ct))
            return Results.Problem("This fruit is sold out or no longer available.", statusCode: 409);
        name = reader.GetString(0);
        price = reader.GetDecimal(1);
    }
    var receipt = new Receipt(Guid.NewGuid(), name, price);
    await using var insert = new NpgsqlCommand(
        "INSERT INTO orders (id,product_id,product_name,total) VALUES ($1,$2,$3,$4)", connection, transaction);
    insert.Parameters.AddWithValue(receipt.Id);
    insert.Parameters.AddWithValue(input.ProductId);
    insert.Parameters.AddWithValue(name);
    insert.Parameters.AddWithValue(price);
    await insert.ExecuteNonQueryAsync(ct);
    await transaction.CommitAsync(ct);
    app.Logger.LogInformation("Order {OrderId} placed for {ProductName}; total {Total}",
        receipt.Id, name, price);
    return Results.Ok(receipt);
});

app.Run();

record Product(Guid Id, string Name, string Description, string Icon, decimal Price, int Stock);
record NewProduct(string Name, string Description, string Icon, decimal Price, int Stock);
record Purchase(Guid ProductId);
record Receipt(Guid Id, string ProductName, decimal Total);
