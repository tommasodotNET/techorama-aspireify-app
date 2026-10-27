targetScope = 'subscription'

param resourceGroupName string

param location string

param principalId string

resource rg 'Microsoft.Resources/resourceGroups@2023-07-01' = {
  name: resourceGroupName
  location: location
}

module aca_acr 'aca-acr/aca-acr.bicep' = {
  name: 'aca-acr'
  scope: rg
  params: {
    location: location
  }
}

module aca 'aca/aca.bicep' = {
  name: 'aca'
  scope: rg
  params: {
    location: location
    aca_acr_outputs_name: aca_acr.outputs.name
    userPrincipalId: principalId
  }
}

output aca_AZURE_CONTAINER_APPS_ENVIRONMENT_DEFAULT_DOMAIN string = aca.outputs.AZURE_CONTAINER_APPS_ENVIRONMENT_DEFAULT_DOMAIN

output aca_AZURE_CONTAINER_APPS_ENVIRONMENT_ID string = aca.outputs.AZURE_CONTAINER_APPS_ENVIRONMENT_ID

output aca_AZURE_CONTAINER_REGISTRY_ENDPOINT string = aca.outputs.AZURE_CONTAINER_REGISTRY_ENDPOINT

output aca_AZURE_CONTAINER_REGISTRY_MANAGED_IDENTITY_ID string = aca.outputs.AZURE_CONTAINER_REGISTRY_MANAGED_IDENTITY_ID