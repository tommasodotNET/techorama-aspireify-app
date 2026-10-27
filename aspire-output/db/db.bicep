@description('The location for the resource(s) to be deployed.')
param location string = resourceGroup().location

param aca_outputs_azure_container_apps_environment_default_domain string

param aca_outputs_azure_container_apps_environment_id string

@secure()
param db_password_value string

resource db 'Microsoft.App/containerApps@2025-07-01' = {
  name: 'db'
  location: location
  properties: {
    configuration: {
      secrets: [
        {
          name: 'postgres-password'
          value: db_password_value
        }
      ]
      activeRevisionsMode: 'Single'
      ingress: {
        external: false
        targetPort: 5432
        transport: 'tcp'
      }
    }
    environmentId: aca_outputs_azure_container_apps_environment_id
    template: {
      containers: [
        {
          image: 'docker.io/library/postgres:18.3'
          name: 'db'
          env: [
            {
              name: 'POSTGRES_HOST_AUTH_METHOD'
              value: 'scram-sha-256'
            }
            {
              name: 'POSTGRES_INITDB_ARGS'
              value: '--auth-host=scram-sha-256 --auth-local=scram-sha-256'
            }
            {
              name: 'POSTGRES_USER'
              value: 'postgres'
            }
            {
              name: 'POSTGRES_PASSWORD'
              secretRef: 'postgres-password'
            }
          ]
        }
      ]
      scale: {
        minReplicas: 1
      }
    }
  }
}