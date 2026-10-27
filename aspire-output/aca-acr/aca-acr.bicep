@description('The location for the resource(s) to be deployed.')
param location string = resourceGroup().location

resource aca_acr 'Microsoft.ContainerRegistry/registries@2025-04-01' = {
  name: take('acaacr${uniqueString(resourceGroup().id)}', 50)
  location: location
  sku: {
    name: 'Basic'
  }
  tags: {
    'aspire-resource-name': 'aca-acr'
  }
}

output name string = aca_acr.name

output loginServer string = aca_acr.properties.loginServer

output id string = aca_acr.id