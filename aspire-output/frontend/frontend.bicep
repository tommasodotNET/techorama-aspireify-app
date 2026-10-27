@description('The location for the resource(s) to be deployed.')
param location string = resourceGroup().location

param aca_outputs_azure_container_apps_environment_default_domain string

param aca_outputs_azure_container_apps_environment_id string

param frontend_containerimage string

param aca_outputs_azure_container_registry_endpoint string

param aca_outputs_azure_container_registry_managed_identity_id string

resource frontend 'Microsoft.App/containerApps@2025-07-01' = {
  name: 'frontend'
  location: location
  properties: {
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: {
        external: false
        targetPort: 5000
        transport: 'http'
      }
      registries: [
        {
          server: aca_outputs_azure_container_registry_endpoint
          identity: aca_outputs_azure_container_registry_managed_identity_id
        }
      ]
    }
    environmentId: aca_outputs_azure_container_apps_environment_id
    template: {
      containers: [
        {
          image: frontend_containerimage
          name: 'frontend'
          env: [
            {
              name: 'NODE_ENV'
              value: 'production'
            }
            {
              name: 'PORT'
              value: '5000'
            }
            {
              name: 'API_UPSTREAM'
              value: 'https://api.internal.${aca_outputs_azure_container_apps_environment_default_domain}'
            }
            {
              name: 'YARP_ENABLE_STATIC_FILES'
              value: 'true'
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