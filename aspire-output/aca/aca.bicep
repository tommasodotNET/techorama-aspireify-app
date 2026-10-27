@description('The location for the resource(s) to be deployed.')
param location string = resourceGroup().location

param userPrincipalId string = ''

param tags object = { }

param aca_acr_outputs_name string

resource aca_mi 'Microsoft.ManagedIdentity/userAssignedIdentities@2024-11-30' = {
  name: take('aca_mi-${uniqueString(resourceGroup().id)}', 128)
  location: location
  tags: tags
}

resource aca_acr 'Microsoft.ContainerRegistry/registries@2025-04-01' existing = {
  name: aca_acr_outputs_name
}

resource aca_acr_aca_mi_AcrPull 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(aca_acr.id, aca_mi.id, subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d'))
  properties: {
    principalId: aca_mi.properties.principalId
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d')
    principalType: 'ServicePrincipal'
  }
  scope: aca_acr
}

resource aca_law 'Microsoft.OperationalInsights/workspaces@2025-02-01' = {
  name: take('acalaw-${uniqueString(resourceGroup().id)}', 63)
  location: location
  properties: {
    sku: {
      name: 'PerGB2018'
    }
  }
  tags: tags
}

resource aca 'Microsoft.App/managedEnvironments@2025-07-01' = {
  name: take('aca${uniqueString(resourceGroup().id)}', 24)
  location: location
  properties: {
    appLogsConfiguration: {
      destination: 'log-analytics'
      logAnalyticsConfiguration: {
        customerId: aca_law.properties.customerId
        sharedKey: aca_law.listKeys().primarySharedKey
      }
    }
    workloadProfiles: [
      {
        name: 'consumption'
        workloadProfileType: 'Consumption'
      }
    ]
  }
  tags: tags
}

resource aspireDashboard 'Microsoft.App/managedEnvironments/dotNetComponents@2025-10-02-preview' = {
  name: 'aspire-dashboard'
  properties: {
    componentType: 'AspireDashboard'
  }
  parent: aca
}

output AZURE_LOG_ANALYTICS_WORKSPACE_NAME string = aca_law.name

output AZURE_LOG_ANALYTICS_WORKSPACE_ID string = aca_law.id

output AZURE_CONTAINER_REGISTRY_NAME string = aca_acr.name

output AZURE_CONTAINER_REGISTRY_ENDPOINT string = aca_acr.properties.loginServer

output AZURE_CONTAINER_REGISTRY_MANAGED_IDENTITY_ID string = aca_mi.id

output AZURE_CONTAINER_APPS_ENVIRONMENT_NAME string = aca.name

output AZURE_CONTAINER_APPS_ENVIRONMENT_ID string = aca.id

output AZURE_CONTAINER_APPS_ENVIRONMENT_DEFAULT_DOMAIN string = aca.properties.defaultDomain