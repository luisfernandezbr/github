import React from 'react';
import { SimulatorInstaller, Integration } from '@pinpt/agent.websdk';
import IntegrationUI from './integration';

function App() {
	// check to see if we are running local and need to run in simulation mode
	if (window === window.parent && window.location.href.indexOf('localhost') > 0) {
		const integration: Integration = {
			name: 'GitHub',
			description: 'The official GitHub integration for Pinpoint',
			tags: ['Source Code Management', 'Issue Management'],
			installed: false,
			refType: 'github',
			icon: 'https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png',
			publisher: {
				name: 'Pinpoint',
				avatar: 'https://avatars0.githubusercontent.com/u/24400526?s=200&v=4',
				url: 'https://pinpoint.com'
			},
			uiURL: 'http://localhost:3000'
		};
		return <SimulatorInstaller integration={integration} />;
	}
	return (
		<div className="App">
			<IntegrationUI />
		</div>
	);
}

export default App;
