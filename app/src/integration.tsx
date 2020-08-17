import React, { useEffect, useState, useRef } from 'react';
import { Icon, Loader, Error as ErrorMessage } from '@pinpt/uic.next';
import {
	useIntegration,
	Account,
	AccountsTable,
	IntegrationType,
	OAuthConnect,
	Graphql,
	Form,
	FormType,
	IAppBasicAuth,
} from '@pinpt/agent.websdk';
import styles from './styles.module.less';

const viewerOrgsGQL = `{
	viewer {
		id
		name
		login
		description: bio
		avatarUrl
		repositories(isFork:false) {
			totalCount
		}
		organizations(first: 100) {
			nodes {
				id
				name
				description
				avatarUrl
				login
				repositories(isFork:false) {
					totalCount
				}
			}
		}
	}
}`;

const githubUserToAccount = (data: any, isPublic: boolean): Account => {
	return {
		id: data.login,
		name: data.name,
		description: data.bio || data.description,
		avatarUrl: data.avatarUrl || data.avatar_url,
		totalCount: data.public_repos || data.repositories?.totalCount || 0,
		type: 'USER',
		public: isPublic,
	}
};

const githubOrgToAccount = (data: any, isPublic: boolean): Account => {
	return {
		id: data.login,
		name: data.name,
		description: data.description,
		avatarUrl: data.avatarUrl || data.avatar_url,
		totalCount: data.public_repos || data.repositories?.totalCount || 0,
		type: 'ORG',
		public: isPublic,
	};
};

const fetchViewerOrgsOAuth = async(api_key: string) => {
	const [data] = await Graphql.query('https://api.github.com/graphql', viewerOrgsGQL, undefined, {Authorization: `Bearer ${api_key}`}, false);
	return data;
};

const fetchViewerOrgsBasic = async(auth: IAppBasicAuth) => {
	const enc = btoa(auth.username + ":" + auth.password);
	const [data] = await Graphql.query(auth.url!, viewerOrgsGQL, undefined, {Authorization: `Basic ${enc}`}, false);
	return data;
};

const AccountList = () => {
	const { processingDetail, config, setConfig, installed, setInstallEnabled } = useIntegration();
	const [accounts, setAccounts] = useState<Account[]>([]);

	useEffect(() => {
		let data: any;
		const fetch = async () => {
			if (config.integration_type === IntegrationType.CLOUD) {
				data = await fetchViewerOrgsOAuth(config.oauth2_auth?.access_token!);
			} else {
				data = await fetchViewerOrgsBasic(config.basic_auth!);
			}
			const orgs = config.accounts || {};
			config.accounts = orgs;
			const newaccounts = data.viewer.organizations.nodes.map((org: any) => githubOrgToAccount(org, false));
			newaccounts.unshift(githubUserToAccount(data.viewer, false));

			if (!installed) {
				newaccounts.forEach((account: Account) => (orgs[account.id] = account));
			}

			Object.keys(orgs).forEach((id: string) => {
				const found = newaccounts.find((acct: Account) => acct.id === id);

				if (!found) {
					const entry = orgs[id];
					newaccounts.push(entry);
				}
			});

			setAccounts(newaccounts);
			setInstallEnabled(installed ? true : Object.keys(config.accounts).length > 0);
			setConfig(config);
		};
		fetch();
	}, [installed, setInstallEnabled, config, setConfig]);

	return (processingDetail?.throttled && processingDetail?.throttledUntilDate) ? (
		<ErrorMessage heading="Your authorization token has been throttled by GitHub." message={`Please try again in ${Math.ceil((processingDetail.throttledUntilDate - Date.now()) / 60000)} minutes.`} />
	) : (
		<AccountsTable
			description="For the selected accounts, all repositories, issues, pull requests and other data will automatically be made available in Pinpoint once installed."
			accounts={accounts}
			entity="repo"
			config={config}
		/>
	);
};

const LocationSelector = ({ setType }: { setType: (val: IntegrationType) => void }) => {
	return (
		<div className={styles.Location}>
			<div className={styles.Button} onClick={() => setType(IntegrationType.CLOUD)}>
				<Icon icon={['fas', 'cloud']} className={styles.Icon} />
				I'm using the <strong>GitHub.com</strong> cloud service to manage my data
			</div>

			<div className={styles.Button} onClick={() => setType(IntegrationType.SELFMANAGED)}>
				<Icon icon={['fas', 'server']} className={styles.Icon} />
				I'm using <strong>my own systems</strong> or a <strong>third-party</strong> to manage a GitHub service
			</div>
		</div>
	);
};

const SelfManagedForm = ({ callback }: any) => {
	const form = {
		basic: {
			password: {
				display: 'Personal Access Token',
				help: 'Please enter your Personal Access Token for this user',
			},
		},
	};
	return (
		<Form type={FormType.BASIC} form={form} name="GitHub" callback={async (auth: IAppBasicAuth) => {
			let url = auth.url!;
			const u = new URL(url!);
			if (url?.indexOf('/graphql') < 0) {
				u.pathname = '/graphql';
				auth.url = u.toString();
			}
			const enc = btoa(auth.username + ":" + auth.password);
			const [data, status] = await Graphql.query(auth.url!, `query { viewer { id } }`, undefined, {Authorization: `Basic ${enc}`});
			if (status !== 200) {
				throw new Error(data.message ?? 'Invalid Credentials');
			}
			callback();
		}} />
	);
};

const Integration = () => {
	const { loading, currentURL, config, isFromRedirect, isFromReAuth, setConfig, authorization } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [, setRerender] = useState(0);
	const currentConfig = useRef(config);

	useEffect(() => {
		if (!loading && authorization?.oauth2_auth) {
			config.integration_type = IntegrationType.CLOUD;
			config.oauth2_auth = {
				date_ts: Date.now(),
				access_token: authorization.oauth2_auth.access_token,
				refresh_token: authorization.oauth2_auth.refresh_token,
				scopes: authorization.oauth2_auth.scopes,
			};

			setType(IntegrationType.CLOUD);
			setConfig(config);

			currentConfig.current = config;
		}
	}, [loading, authorization]);

	useEffect(() => {
		if (!loading && isFromRedirect && currentURL) {
			const search = currentURL.split('?');

			const tok = search[1].split('&');
			tok.forEach(token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];

				if (k === 'profile') {
					const profile = JSON.parse(atob(decodeURIComponent(v)));
					config.integration_type = IntegrationType.CLOUD;
					config.oauth2_auth = {
						date_ts: Date.now(),
						access_token: profile.Integration.auth.accessToken,
						scopes: profile.Integration.auth.scopes,
					};

					setType(IntegrationType.CLOUD);
					setConfig(config);

					currentConfig.current = config;
				}
			});
		}
	}, [loading, isFromRedirect, currentURL, config, setRerender, setConfig]);

	useEffect(() => {
		if (type) {
			config.integration_type = type;
			currentConfig.current = config;

			setConfig(config);
			setRerender(Date.now());
		}
	}, [type]);

	if (loading) {
		return <Loader screen />;
	}

	let content;

	if (isFromReAuth) {
		if (config.integration_type === IntegrationType.CLOUD) {
			content = <OAuthConnect name="GitHub" reauth />
		} else {
			content = <SelfManagedForm />;
		}
	} else {
		if (!config.integration_type) {
			content = <LocationSelector setType={setType} />;
		} else if (config.integration_type === IntegrationType.CLOUD && !config.oauth2_auth) {
			content = <OAuthConnect name="GitHub" />;
		} else if (config.integration_type === IntegrationType.SELFMANAGED) {
			if (!config?.basic_auth?.url) {
				content = <SelfManagedForm callback={() => setRerender(Date.now())} />;
			} else {
				content = <AccountList />;
			}
		} else {
			content = <AccountList />;
		}
	}

	return (
		<div className={styles.Wrapper}>
			{content}
		</div>
	)
};

export default Integration;