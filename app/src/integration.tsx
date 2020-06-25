import React, { useCallback, useEffect, useState, useRef } from 'react';
import { useIntegration } from '@pinpt/agent.websdk';
import { Button, Dialog, Icon, ListPanel, Theme } from '@pinpt/uic.next';

const AuthorizationRequired = () => {
	const { setRedirectTo, getRedirectURL, getAppOAuthURL } = useIntegration();
	const onClick = async () => {
		const theurl = await getRedirectURL();
		const redirectTo = await getAppOAuthURL(theurl);
		setRedirectTo(redirectTo);
	};
	return (
		<div style={{display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column'}}>
			<div style={{fontSize: '1.6rem', margin: '3rem 6rem', fontWeight: 'bold'}}>
				To begin, we will need to redirect to GitHub to grant permission to Pinpoint to use your
				GitHub data. Once you grant permission, GitHub will return you back to this screen to
				complete your configuration and then install this integration.
			</div>
			<Button color="Blue" weight={500} onClick={onClick}>Connect Pinpoint to GitHub</Button>
		</div>
	);
};

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

interface CheckboxProps {
	className?: string;
	checked: boolean;
	onChange?: (checked: boolean) => void;
	disabled?: boolean;
}

const Checkbox = ({
	className,
	onChange,
	checked,
	disabled,
}: CheckboxProps) => {
	const onchange = () => {
		if (onChange) {
			onChange(!checked);
		}
	};
	return (
		<input
			className={className}
			type="checkbox"
			checked={checked}
			onChange={onchange}
			disabled={disabled ?? false}
		/>
	);
};

interface Account {
	id: string;
	name: string;
	description: string;
	avatarUrl: string;
	login: string;
	repositories: {
		totalCount: number;
	};
	type: 'USER' | 'ORG';
	public: boolean;
}

const AccountSelector = ({account, config}: {account: Account, config: {[key: string]: any}}) => {
	const { setConfig, setInstallEnabled, installed } = useIntegration();
	const [selected, setSelected] = useState(true);
	const onChange = useCallback((val: boolean) => {
		setSelected(val);
		const accounts = config.accounts || {};
		if (val) {
			accounts[account.login] = val;
		} else {
			delete accounts[account.login];
		}
		config.accounts = accounts;
		setInstallEnabled(installed ? true : Object.keys(accounts).length > 0);
		setConfig(config);
	}, [setSelected, setConfig, setInstallEnabled, account, config, installed]);
	return (
		<div style={{display: 'flex', flexDirection: 'row', alignItems: 'center'}}>
			<Checkbox checked={selected} onChange={onChange} />
			<img alt="" style={{marginLeft: '1rem'}} src={account.avatarUrl} width={20} height={20} />
			<span style={{marginLeft: '1rem'}}>{account.name}</span>
		</div>
	);
};

const buildAccountRow = (account: Account, config: {[key: string]: any}, onClick: (account: Account) => void) => {
	const accounts = config.accounts || {};
	accounts[account.login] = account;
	return {
		key: account.login,
		left: <AccountSelector account={account} config={config} />,
		center: <span style={{marginLeft: '1rem', color: Theme.Royal300}}>{account.description}</span>,
		right: <span style={{color: Theme.Mono500}}>{new Intl.NumberFormat().format(account.repositories.totalCount)} repos <Button onClick={() => onClick(account)} style={{marginLeft: '1rem', padding: '1px 5px'}}>+ Exclusions</Button></span>,
	};
};

const AskDialog = ({title, text, buttonOK, show, onChange, onCancel, onSubmit, textarea, value}: {title: string, text: string, buttonOK: string, show: boolean, onChange: (val: string) => void, onCancel: () => void, onSubmit: () => void, textarea: boolean, value: string}) => {
	const ref = useRef<any>();
	const onChangeHandler = (evt: any) => onChange(evt.target.value);
	useEffect(() => {
		if (show && ref?.current) {
			ref.current.focus();
			ref.current.select();
		}
	}, [ref, show]);
	let field: React.ReactElement;
	if (textarea) {
		field = (
			<textarea
				ref={ref}
				rows={5}
				style={{
					width: '450px',
					fontSize: '1.5rem',
					padding: '2px',
					outline: 'none',
				}}
				value={value}
				onChange={onChangeHandler} />
		);
	} else {
		field = (
			<input
				ref={ref}
				type="text"
				value={value}
				style={{
					width: '450px',
					fontSize: '1.5rem',
					padding: '2px',
					outline: 'none',
				}}
				onChange={onChangeHandler}
			/>
		);
	}
	return (
		<Dialog open={show} style={{background: 'transparent'}} centered={false}>
			<h1>{title}</h1>
			<p>
				{text}
			</p>
			<p>
				{field}
			</p>
			<div className="buttons">
				<Button color="Mono" weight={500} onClick={onCancel}>Cancel</Button>
				<Button color="Green" weight={500} onClick={onSubmit}>
					<>{buttonOK}</>
				</Button>
			</div>
		</Dialog>	
	);
};

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

const fetchUser = (name: string, api_key: string, onAdd: (account: Account) => void) => {
	const xhr = new XMLHttpRequest();
	xhr.open('GET', 'https://api.github.com/users/' + getEntityName(name));
	xhr.setRequestHeader('Content-Type', 'application/json');
	xhr.setRequestHeader('Authorization', `Bearer ${api_key}`);
	xhr.responseType = 'json';
	xhr.onreadystatechange = function() {
		if (this.readyState === 4) {
			const data = this.response;
			switch (this.status) {
				case 404: {
					// user not found
					break;
				}
				case 200: {
					onAdd({
						id: data.node_id,
						login: data.login,
						name: data.name,
						description: data.bio,
						avatarUrl: data.avatar_url,
						repositories: {
							totalCount: data.public_repos,
						},
						type: 'USER',
						public: true,
					});
					break;
				}
			}
		}
	};
	xhr.send();
};


const fetchOrg = (name: string, api_key: string, onAdd: (account: Account) => void) => {
	const xhr = new XMLHttpRequest();
	xhr.open('GET', 'https://api.github.com/orgs/' + getEntityName(name));
	xhr.setRequestHeader('Content-Type', 'application/json');
	xhr.setRequestHeader('Authorization', `Bearer ${api_key}`);
	xhr.responseType = 'json';
	xhr.onreadystatechange = function() {
		if (this.readyState === 4) {
			const data = this.response;
			switch (this.status) {
				case 404: {
					// org not found, check and see if maybe a user?
					fetchUser(name, api_key, onAdd);
					break;
				}
				case 200: {
					onAdd({
						id: data.id,
						login: data.login,
						name: data.name,
						description: data.description,
						avatarUrl: data.avatar_url,
						repositories: {
							totalCount: data.public_repos,
						},
						type: 'ORG',
						public: true,
					});
					break;
				}
			}
		}
	};
	xhr.send();
};

const ShowAccounts = ({api_key, config}: {api_key: string, config: {[key: string]: any}}) => {
	const { setConfig, setInstallEnabled, installed } = useIntegration();
	const [accounts, setAccounts] = useState<any[]>([]);
	const [account, setAccount] = useState<Account>();
	const [exclusions, setExclusions] = useState('');
	const [publicAcct, setPublicAcct] = useState('');
	const [showAddAccountModal, setShowAddAccountModal] = useState(false);
	const doShowAddAccountModal = useCallback(() => setShowAddAccountModal(true), []);
	const [showAddExclusionModal, setShowAddExclusionModal] = useState(false);
	const doShowAddExclusionModal = useCallback((account: Account) => {
		setShowAddExclusionModal(true);
		setAccount(account);
		setExclusions((config.exclusions || {})[account.login] || '');
	}, [config, setExclusions, setAccount, setShowAddExclusionModal]);
	useEffect(() => {
		if (api_key) {
			const xhr = new XMLHttpRequest();
			xhr.open('POST', 'https://api.github.com/graphql');
			xhr.setRequestHeader('Content-Type', 'application/json');
			xhr.setRequestHeader('Authorization', `Bearer ${api_key}`);
			xhr.responseType = 'json';
			xhr.onreadystatechange = function() {
				if (this.readyState === 4 && this.status === 200) {
					// console.log(this.response.data.viewer);
					const orgs = config.accounts || {};
					config.accounts = orgs;
					const newaccounts = this.response.data.viewer.organizations.nodes.map((org: any) => buildAccountRow({...org, type: 'ORG', public: false}, config, doShowAddExclusionModal));
					newaccounts.unshift(buildAccountRow({...this.response.data.viewer, type: 'USER', public: false}, config, doShowAddExclusionModal));
					Object.keys(orgs).forEach((login: string) => {
						const found = newaccounts.find((org: any) => org.key === login);
						if (!found) {
							newaccounts.push(buildAccountRow(orgs[login], config, doShowAddExclusionModal));
						}
					});
					setAccounts(newaccounts);
					setInstallEnabled(installed ? true : Object.keys(config.accounts).length > 0);
					setConfig(config);
				}
				// FIXME: handle error
			};
			xhr.send(JSON.stringify({query: viewerOrgsGQL}));
		}
	}, [api_key, config, setAccounts, setInstallEnabled, setConfig, doShowAddExclusionModal, installed]);
	return (
		<>
			<div style={{display: 'flex', flexDirection: 'row', alignItems: 'center', justifyContent: 'center'}}>
				<div style={{fontSize: '1.4rem', marginBottom: '2rem'}}>
					The selected accounts will be managed by Pinpoint. All repositories, issues, pull requests and other data will automatically be made available in Pinpoint once installed.
				</div>
				<span style={{marginLeft: 'auto', marginBottom: '1rem'}}><Button onClick={doShowAddAccountModal}>+ Public Account</Button></span>
			</div>
			<ListPanel title="Accounts" rows={accounts} empty={<>No accounts found</>} />
			<AskDialog
				textarea={true}
				value={exclusions}
				show={showAddExclusionModal}
				title="Add a set of ignore rules"
				text="Enter repo exclusion rules using .gitignore pattern:"
				buttonOK="Add"
				onCancel={() => setShowAddExclusionModal(false)}
				onChange={(val: string) => setExclusions(val)}
				onSubmit={() => {
					const excl = config.exclusions || {};
					excl[account?.login!] = exclusions;
					config.exclusions = excl;
					setShowAddExclusionModal(false);
					setAccount(undefined);
					setExclusions('');
					setConfig(config);
				}}
			/>
			<AskDialog
				textarea={false}
				show={showAddAccountModal}
				title="Add a Public Account"
				text="What is the login or URL to the GitHub account?"
				buttonOK="Add"
				value={publicAcct}
				onCancel={() => setShowAddAccountModal(false)}
				onChange={(val: string) => setPublicAcct(val)}
				onSubmit={() => {
					// TODO: parse out url
					setShowAddAccountModal(false);
					const val = publicAcct;
					setPublicAcct('');
					fetchOrg(val, api_key, (account: Account) => {
						const newaccts = [...accounts, buildAccountRow(account, config, doShowAddExclusionModal)];
						setAccounts(newaccts);
						setInstallEnabled(true);
						setConfig(config);
					});
				}}
			/>
		</>
	);
};

const chooseIntegrationStyles: any = {
	container: {
		fontSize: '1.6rem',
		display:'flex',
		alignItems: 'center',
		justifyContent: 'center',
		flexDirection: 'row',
		width: '100%',
		height: '400px',
		cursor: 'pointer',
	},
	button: {
		border: '1px solid #eee',
		backgroundColor: Theme.Blue500,
		borderRadius: '8px',
		height: '200px',
		width: '400px',
		margin: '2rem',
		textAlign: 'center',
		padding: '2rem',
		color: Theme.Mono100,
		fontWeight: 'bold',
		fontSize: '1.7rem',
	},
	icon: {
		fontSize: '10rem',
		marginBottom: '2rem',
		color: Theme.Mono100,
	}
};

const ChooseIntegrationType = ({ setType }: { setType: (val: any) => void }) => {
	return (
		<div style={chooseIntegrationStyles.container}>
			<div style={chooseIntegrationStyles.button} onClick={() => setType('CLOUD')}>
				<div><Icon icon={['fas', 'cloud']} style={chooseIntegrationStyles.icon} /></div>
				<div>I'm using GitHub.com to manage my data using their cloud service</div>
			</div>
			<div style={chooseIntegrationStyles.button} onClick={() => setType('SELFMANAGED')}>
				<div><Icon icon={['fas', 'home']} style={chooseIntegrationStyles.icon} /></div>
				I'm using GitHub on my own systems or using a third-party managed GitHub service
			</div>
		</div>
	);
};

const SelfManagedForm = ({ config } : { config: {[key: string]: any} }) => {
	const { setInstallEnabled, setConfig } = useIntegration();
	const [url, setURL] = useState(config.selfmanaged?.url);
	const [apikey, setAPIKey] = useState(config.selfmanaged?.apikey);
	const urlRef = useRef<any>();
	const apikeyRef = useRef<any>();
	const onUrlChange = useCallback(() => {
		const props = config.selfmanaged || {};
		config.selfmanaged = props;
		props.url = urlRef.current.value;
		setURL(urlRef.current.value);
		setInstallEnabled(props.url && props.apikey);
		setConfig(config);
	}, [config, setURL, setInstallEnabled, setConfig]);
	const onAPIKeyChange = useCallback(() => {
		const props = config.selfmanaged || {};
		config.selfmanaged = props;
		props.apikey = apikeyRef.current.value;
		setAPIKey(apikeyRef.current.value);
		setInstallEnabled(props.url && props.apikey);
		setConfig(config);
	}, [config, setAPIKey, setInstallEnabled, setConfig]);
	useEffect(() => {
		const props = config.selfmanaged || {};
		setInstallEnabled(props.url && props.apikey);
	}, [config, setInstallEnabled]);
	return (
		<div style={{fontSize: '1.6rem'}}>
			<p style={{marginBottom: '2rem'}}>Enter your credentials to your GitHub server</p>
			<div style={{display: 'flex', flexDirection:'row', flexWrap: 'wrap', alignItems: 'center', marginBottom: '2rem'}}>
				<label style={{flex:'1 0 2rem', maxWidth: '10rem'}}>URL</label>
				<input ref={urlRef} style={{flex:'1 0 2rem', maxWidth: '50rem'}} type="text" name="url" value={url} placeholder="Your GitHub URL" onChange={onUrlChange} />
			</div>
			<div style={{display: 'flex', flexDirection:'row'}}>
				<label style={{flex:'1 0 2rem', maxWidth: '10rem'}}>API Key</label>
				<input ref={apikeyRef} style={{flex:'1 0 2rem', maxWidth: '50rem'}} type="text" name="apikey" value={apikey} placeholder="Your GitHub API Key" onChange={onAPIKeyChange} />
			</div>
		</div>
	);
};

const Integration = () => {
	const { currentURL, config, isFromRedirect, setConfig } = useIntegration();
	const [type, setType] = useState<'CLOUD'|'SELFMANAGED'>(config.integrationType);
	const [, setRerender] = useState(0);
	const currentConfig = useRef(config);
	useEffect(() => {
		if (isFromRedirect && currentURL) {
			const search = currentURL.split('?');
			const tok = search[1].split('&');
			tok.forEach(token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];
				if (k === 'token') {
					config.api_key = v;
					setType('CLOUD');
					setConfig(config);
					setRerender(Date.now());
					currentConfig.current = config;
				} else if (k === 'profile') {
					config.profile = JSON.parse(atob(decodeURIComponent(v)));
					setType('CLOUD');
					setConfig(config);
					setRerender(Date.now());
					currentConfig.current = config;
				}
			});
		}
	}, [isFromRedirect, currentURL, config, setRerender, setType, setConfig]);
	useEffect(() => {
		if (type) {
			config.integrationType = type;
			currentConfig.current = config;
			setConfig(config);
			setRerender(Date.now());
		}
	}, [config, type, setConfig, setRerender]);
	console.log(config);
	if (!config.integrationType) {
		return <ChooseIntegrationType setType={setType} />;
	}
	if (!config.profile && config.integrationType === 'CLOUD') {
		return <AuthorizationRequired />;
	}
	if (config.integrationType === 'SELFMANAGED') {
		return <SelfManagedForm config={config} />;
	}
	return (
		<ShowAccounts api_key={config.api_key} config={config} />
	);
};

export default Integration;