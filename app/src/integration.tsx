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
	ConfigAccount,
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


const toAccount = (data: ConfigAccount): Account => {
	return {
		id: data.id,
		public: data.public,
		type: data.type,
		avatarUrl: data.avatarUrl,
		name: data.name || '',
		description: data.description || '',
		totalCount: data.totalCount || 0,
		selected: !!data.selected
	}
};


interface validationResponse {
	accounts: ConfigAccount[];
}

const AccountList = () => {
	const { processingDetail, config, setConfig, installed, setInstallEnabled, setValidate } = useIntegration();
	const [accounts, setAccounts] = useState<Account[]>([]);
	useEffect(() => {
		const fetch = async () => {
			let data: validationResponse;
			data = await setValidate(config);
			config.accounts = config.accounts || {};
			var accounts = data.accounts as Account[];
			accounts.forEach(( account ) => {
				if ( config  && config.accounts){
					const selected = config.accounts[account.id]?.selected
					if (installed) {	
						account.selected = !!selected
					}
					config.accounts[account.id] = account;
				}
			});
			setAccounts(data.accounts.map((acct) => toAccount(acct)));
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
			const [data, status] = await Graphql.query(auth.url!, `query { viewer { id } }`, undefined, { Authorization: `Basic ${enc}` });
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