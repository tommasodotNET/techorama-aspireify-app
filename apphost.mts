// Aspire TypeScript AppHost
// For more information, see: https://aspire.dev

import { createBuilder } from './.aspire/modules/aspire.mjs';

const builder = await createBuilder();

await builder.addViteApp('slides', './slides', { runScriptName: 'start' })
    .withUrlForEndpoint('http', async url => {
        url.url = '/aspireify/index.html';
        url.displayText = 'Slides';
    });

await builder.build().run();