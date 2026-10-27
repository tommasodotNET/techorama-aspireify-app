@description('The location for the resource(s) to be deployed.')
param location string = resourceGroup().location

param aca_outputs_azure_container_apps_environment_default_domain string

param aca_outputs_azure_container_apps_environment_id string

param api_containerimage string

param api_containerport string

param websiteadmin_value string

@secure()
param db_password_value string

param aca_outputs_azure_container_registry_endpoint string

param aca_outputs_azure_container_registry_managed_identity_id string

resource api 'Microsoft.App/containerApps@2025-10-02-preview' = {
  name: 'api'
  location: location
  properties: {
    configuration: {
      secrets: [
        {
          name: 'connectionstrings--store'
          value: 'Host=db;Port=5432;Username=postgres;Password=${db_password_value};Database=store'
        }
        {
          name: 'store-password'
          value: db_password_value
        }
        {
          name: 'store-uri'
          value: 'postgresql://postgres:${uriComponent(db_password_value)}@db:5432/store'
        }
      ]
      activeRevisionsMode: 'Single'
      ingress: {
        external: false
        targetPort: int(api_containerport)
        transport: 'http'
      }
      registries: [
        {
          server: aca_outputs_azure_container_registry_endpoint
          identity: aca_outputs_azure_container_registry_managed_identity_id
        }
      ]
      runtime: {
        dotnet: {
          autoConfigureDataProtection: true
        }
      }
    }
    environmentId: aca_outputs_azure_container_apps_environment_id
    template: {
      containers: [
        {
          image: api_containerimage
          name: 'api'
          env: [
            {
              name: 'OTEL_DOTNET_EXPERIMENTAL_OTLP_RETRY'
              value: 'in_memory'
            }
            {
              name: 'ASPNETCORE_FORWARDEDHEADERS_ENABLED'
              value: 'true'
            }
            {
              name: 'HTTP_PORTS'
              value: api_containerport
            }
            {
              name: 'Admin__Password'
              value: websiteadmin_value
            }
            {
              name: 'ConnectionStrings__store'
              secretRef: 'connectionstrings--store'
            }
            {
              name: 'STORE_HOST'
              value: 'db'
            }
            {
              name: 'STORE_PORT'
              value: '5432'
            }
            {
              name: 'STORE_USERNAME'
              value: 'postgres'
            }
            {
              name: 'STORE_PASSWORD'
              secretRef: 'store-password'
            }
            {
              name: 'STORE_URI'
              secretRef: 'store-uri'
            }
            {
              name: 'STORE_JDBCCONNECTIONSTRING'
              value: 'jdbc:postgresql://db:5432/store'
            }
            {
              name: 'STORE_DATABASENAME'
              value: 'store'
            }
          ]
        }
      ]
      scale: {
        minReplicas: 1
      }
    }
  }
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${aca_outputs_azure_container_registry_managed_identity_id}': { }
    }
  }
}