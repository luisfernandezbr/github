import React, { useCallback, useEffect, useState, useRef } from 'react';
import { Button, Icon, Theme, Loader } from '@pinpt/uic.next';
import {
	useIntegration,
	Account,
	AccountsTable,
	AskDialog,
	IntegrationType,
	OAuthConnect,
	Graphql,
	Http
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

const getEntityName = (val: string) => {
	let res = val;
	if (res.charAt(0) === '@') {
		res = res.substring(1);
	}
	if (/https?:/.test(res)) {
		// looks like a url
		const i = res.lastIndexOf('/');
		res = res.substring(i + 1);
	}
	return res;
};

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

const fetchUser = async (name: string, api_key: string, onAdd: (account: Account) => void) => {
	const [data, status] = await Http.get('https://api.github.com/users/' + getEntityName(name), {Authorization: `Bearer ${api_key}`});
	if (status === 404) {
		alert(`User ${name} doesn't exist`);
		return;
	}
	onAdd(githubUserToAccount(data, true));
};

const fetchOrg = async (name: string, api_key: string, onAdd: (account: Account) => void) => {
	const [data, status] = await Http.get('https://api.github.com/orgs/' + getEntityName(name), {Authorization: `Bearer ${api_key}`});
	if (status === 404) {
		fetchUser(name, api_key, onAdd);
		return;
	}
	onAdd(githubOrgToAccount(data, true));
};

const fetchViewerOrgs = async(api_key: string) => {
	const [data] = await Graphql.query('https://api.github.com/graphql', viewerOrgsGQL, undefined, {Authorization: `Bearer ${api_key}`});
	return data;
};

const ShowAccounts = () => {
	const { config, setConfig, installed, setInstallEnabled } = useIntegration();
	const [accounts, setAccounts] = useState<Account[]>([]);
	useEffect(() => {
		if (config.integration_type === IntegrationType.CLOUD) {
			const fetch = async () => {
				const data = await fetchViewerOrgs(config.oauth2_auth?.access_token!);
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
		}
	}, []);
	const [publicAcct, setPublicAcct] = useState('');
	const [showAddAccountModal, setShowAddAccountModal] = useState(false);
	const doShowAddAccountModal = useCallback(() => setShowAddAccountModal(true), []);
	return (
		<>
			<AccountsTable
				title="The selected accounts will be managed by Pinpoint. All repositories, issues, pull requests and other data will automatically be made available in Pinpoint once installed."
				accounts={accounts}
				entity="repo"
				config={config}
				button={<span style={{marginLeft: 'auto', marginBottom: '1rem'}}><Button onClick={doShowAddAccountModal}>+ Public Account</Button></span>}
			/>
	 		{config.integration_type === IntegrationType.CLOUD && <AskDialog
	 			textarea={false}
	 			show={showAddAccountModal}
	 			title="Add a Public Account"
	 			text="What is the login or URL to the GitHub account?"
	 			buttonOK="Add"
	 			value={publicAcct}
	 			onCancel={() => setShowAddAccountModal(false)}
	 			onChange={(val: string) => setPublicAcct(val)}
	 			onSubmit={() => {
	 				setShowAddAccountModal(false);
	 				const val = publicAcct;
					setPublicAcct('');
					const login = getEntityName(val);
	 				fetchOrg(login, config.oauth2_auth?.access_token!, (account: Account) => {
						const found = accounts.find((acct: Account) => account.id === acct.id);
						if (found) {
							return;
						}
						const _accounts = [account, ...accounts];
						setAccounts(_accounts);
						config.accounts![account.id] = account;
	 					setInstallEnabled(true);
	 					setConfig(config);
	 				});
				 }}
			/>}
		</>
	);
};

const ChooseIntegrationType = ({ setType }: { setType: (val: IntegrationType) => void }) => {
	return (
		<div className={styles.Container}>
			<div className={styles.Button} onClick={() => setType(IntegrationType.CLOUD)}>
				<div><Icon icon={['fas', 'cloud']} className={styles.Icon} /></div>
				<div>I'm using GitHub.com to manage my data using their cloud service</div>
			</div>
			<div className={styles.Button} onClick={() => setType(IntegrationType.SELFMANAGED)}>
				<div><Icon icon={['fas', 'home']} className={styles.Icon} /></div>
				I'm using GitHub on my own systems or using a third-party managed GitHub service
			</div>
		</div>
	);
};

const SelfManagedForm = () => {
	// const { setInstallEnabled, setConfig } = useIntegration();
	// const [url, setURL] = useState(config.selfmanaged?.url);
	// const [apikey, setAPIKey] = useState(config.selfmanaged?.apikey);
	// const urlRef = useRef<any>();
	// const apikeyRef = useRef<any>();
	// const onUrlChange = useCallback(() => {
	// 	const props = config.selfmanaged || {};
	// 	config.selfmanaged = props;
	// 	props.url = urlRef.current.value;
	// 	setURL(urlRef.current.value);
	// 	setInstallEnabled(props.url && props.apikey);
	// 	setConfig(config);
	// }, [config, setURL, setInstallEnabled, setConfig]);
	// const onAPIKeyChange = useCallback(() => {
	// 	const props = config.selfmanaged || {};
	// 	config.selfmanaged = props;
	// 	props.apikey = apikeyRef.current.value;
	// 	setAPIKey(apikeyRef.current.value);
	// 	setInstallEnabled(props.url && props.apikey);
	// 	setConfig(config);
	// }, [config, setAPIKey, setInstallEnabled, setConfig]);
	// useEffect(() => {
	// 	const props = config.selfmanaged || {};
	// 	setInstallEnabled(props.url && props.apikey);
	// }, [config, setInstallEnabled]);
	// return (
	// 	<div style={{fontSize: '1.6rem'}}>
	// 		<p style={{marginBottom: '2rem'}}>Enter your credentials to your GitHub server</p>
	// 		<div style={{display: 'flex', flexDirection:'row', flexWrap: 'wrap', alignItems: 'center', marginBottom: '2rem'}}>
	// 			<label style={{flex:'1 0 2rem', maxWidth: '10rem'}}>URL</label>
	// 			<input ref={urlRef} style={{flex:'1 0 2rem', maxWidth: '50rem'}} type="text" name="url" value={url} placeholder="Your GitHub URL" onChange={onUrlChange} />
	// 		</div>
	// 		<div style={{display: 'flex', flexDirection:'row'}}>
	// 			<label style={{flex:'1 0 2rem', maxWidth: '10rem'}}>API Key</label>
	// 			<input ref={apikeyRef} style={{flex:'1 0 2rem', maxWidth: '50rem'}} type="text" name="apikey" value={apikey} placeholder="Your GitHub API Key" onChange={onAPIKeyChange} />
	// 		</div>
	// 	</div>
	// );
	return <>TODO</>;
};

const Integration = () => {
	const { loading, currentURL, config, isFromRedirect, setConfig, authorization } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [, setRerender] = useState(0);
	const currentConfig = useRef(config);
	useEffect(() => {
		if (!loading && authorization?.access_token) {
			config.integration_type = IntegrationType.CLOUD;
			config.oauth2_auth = {
				access_token: authorization.access_token,
				refresh_token: authorization.refresh_token,
				scopes: authorization.scopes,
				created: authorization.created,
			};
			setType(IntegrationType.CLOUD);
			setConfig(config);
			currentConfig.current = config;
		}
	}, [loading]);
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
						access_token: profile.Integration.auth.accessToken,
						scopes: profile.Integration.auth.scopes,
						created: profile.Integration.auth.created,
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
		return <Loader />;
	}
	if (!config.integration_type) {
		return <ChooseIntegrationType setType={setType} />;
	}
	if (config.integration_type === IntegrationType.CLOUD && !config.oauth2_auth) {
		return <OAuthConnect name="GitHub" />;
	}
	if (config.integration_type === IntegrationType.SELFMANAGED && (!config.basic_auth || !config.apikey_auth)) {
		return <SelfManagedForm />;
	}
	return (
		<ShowAccounts />
	);
};

export default Integration;