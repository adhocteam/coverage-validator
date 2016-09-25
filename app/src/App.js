import React, { Component } from 'react';
import ReactDOM from 'react-dom';
import './App.css';
import spinnerImg from './spinner.gif';
import 'whatwg-fetch';

//const serverAddr = 'http://localhost:8082'
const serverAddr = '';

const schemas = [
    {value: 'plans', label: 'Plans'},
    {value: 'providers', label: 'Providers'},
    {value: 'drugs', label: 'Drugs'},
    {value: 'index', label: 'Index'},
];

const years = [2016, 2017];

class App extends Component {
    constructor(props) {
        super(props);
        this.state = {
            file: {
                schema: 'plans',
                year: 2017,
                file: null,
                validating: false,
                onChange: this.handleChange('file').bind(this),
                onSubmit: this.handleSubmit('file').bind(this)
            },
            textarea: {
                schema: 'plans',
                year: 2017,
                json: '',
                validating: false,
                onChange: this.handleChange('textarea').bind(this),
                onSubmit: this.handleSubmit('textarea').bind(this)
            },
            validationResult: null
        };

        this.doExample = this.doExample.bind(this);
    }

    componentDidMount() {
        window.addEventListener('scroll', event => {
            if (document.body.scrollTop > 120) {
                document.body.classList.add('fixed-results');
            } else {
                document.body.classList.remove('fixed-results');
            }
        }, false);
    }

    handleChange(type) {
        return (name, value) => {
            this.setStateOnType(type, name, value);
        }
    }

    checkResponse(response) {
        if (response.status >= 200 && response.status < 300) {
            return response;
        }
        var err = new Error(response.statusText);
        err.response = response;
        throw err;
    }

    setStateOnType(type, name, value) {
        let state = this.state[type];
        state[name] = value;
        this.setState(this.state);
    }

    handleSubmit(type) {
        return event => {
            this.setStateOnType(type, "validating", true);
            let data = new FormData();
            let state = this.state[type];
            data.append('schema', state.schema);
            data.append('schemaYear', state.year);
            switch (type) {
                case 'file':
                    data.append('json', state.file);
                    break;
                case 'textarea':
                    data.append('json', state.json);
                    break;
                default:
                    // nop
            }
            fetch(serverAddr + '/validate', {
                method: 'POST',
                body: data
            }).then(this.checkResponse)
              .then(response => response.json())
              .then(json => {
                  this.setState({validationResult: json})
              })
              .catch(err => {
                  this.setState({validationResult: null});
                  console.error(err);
              })
              .then(() => {
                  this.setStateOnType(type, "validating", false);
              });
        };
    }

    doExample() {
        fetch('example.json')
            .then(response => response.text())
            .then(example => {
                let state = this.state.textarea;
                state.schema = 'plans';
                state.year = 2016;
                state.json = example;
                this.setState({textarea: state});
                state.onSubmit(null);
                window.location.href = '#textarea-validator-form';
            })
    }

    render() {
        return (
            <div className="container">
                <CoverageValidator schemas={schemas} years={years}
                                   file={this.state.file}
                                   textarea={this.state.textarea}
                                   validationResult={this.state.validationResult}
                                   onExampleClick={this.doExample} />
                <hr />
                <footer>
                    <p>Dump schemas:</p>
                    <ul>
                        <li><a href="/schema/plans">Plans</a></li>
                        <li><a href="/schema/providers">Providers</a></li>
                        <li><a href="/schema/drugs">Drugs</a></li>
                        <li><a href="/schema/index">Index</a></li>
                    </ul>
                    <hr />
                    <p className="fork"><a href="https://github.com/adhocteam/coverage-validator">Fork this project on GitHub</a></p>
                </footer>
            </div>
        );
    }
}

class CoverageValidator extends Component {
    constructor(props) {
        super(props);
        this.handleExampleClick = this.handleExampleClick.bind(this);
    }

    handleExampleClick(event) {
        event.preventDefault();
        this.props.onExampleClick();
    }

    render() {
        let results;
        if (this.props.validationResult) {
            results = <ValidationResults
                          valid={this.props.validationResult.valid}
                          errors={this.props.validationResult.errors}
                          warnings={this.props.validationResult.warnings}
                          schema={this.props.validationResult.schema}
                          year={this.props.validationResult.year} />;
        }
        let file = this.props.file;
        let textarea = this.props.textarea;
        return (
            <div className="coverage-validator">
                <h2>QHP, provider, and formulary coverage data - JSON schema validator</h2>
                <h3 className="beta">BETA</h3>
                <p>
                    <a href="/docs">Click here for more information</a> | <a href="#helpBlock" className="example" onClick={this.handleExampleClick}>Try an example</a>
                </p>
                <div className="row">
                    <div className="col-md-7">
                        <FileValidatorForm schemas={this.props.schemas} years={this.props.years}
                                           schema={file.schema} year={file.year} file={file.file}
                                           validating={file.validating}
                                           onChange={file.onChange}
                                           onSubmit={file.onSubmit} />
                        <hr />
                        <TextareaValidatorForm schemas={this.props.schemas} years={this.props.years}
                                               schema={textarea.schema} year={textarea.year} json={textarea.json}
                                               validating={textarea.validating}
                                               onChange={textarea.onChange}
                                               onSubmit={textarea.onSubmit} />
                    </div>
                    <div className="col-md-5">
                        {results}
                    </div>
                </div>
            </div>
        );
    }
}

class FormValidator {
    validYear(year) {
        return years.indexOf(parseInt(year, 10)) >= 0;
    }

    validSchema(schema) {
        let found = false;
        schemas.forEach(s => {
            if (s.value === schema) {
                found = true;
            }
        });
        return found;
    }
}

class FileFormValidator extends FormValidator {
    validFile(files) {
        return files.length > 0;
    }
}

class TextareaFormValidator extends FormValidator {
    validJSON(json) {
        return json.length > 0;
    }
}

class FileValidatorForm extends Component {
    constructor(props) {
        super(props);
        this.state = {
            errors: []
        };
        this.handleChange = this.handleChange.bind(this);
        this.handleSubmit = this.handleSubmit.bind(this);

        this.formValidator = new FileFormValidator();
    }

    handleChange(name) {
        return event => {
            let value;
            switch (event.target.type) {
                case 'file':
                    value = event.target.files[0];
                    break;
                default:
                    value = event.target.value;
            }
            this.props.onChange(name, value);
        };
    }

    handleSubmit(event) {
        event.preventDefault();

        this.setState({errors: []});

        let schema = ReactDOM.findDOMNode(this.refs.schema).value;
        let year = ReactDOM.findDOMNode(this.refs.year).value;
        let files = ReactDOM.findDOMNode(this.refs.file).files;

        let errors = [];

        if (!this.formValidator.validYear(year)) errors.push(`invalid year: ${year}`);
        if (!this.formValidator.validSchema(schema)) errors.push(`invalid schema: ${schema}`);
        if (!this.formValidator.validFile(files)) errors.push(`must select a file to validate`);

        if (errors.length > 0) {
            this.setState({errors: errors});
            return;
        }

        this.props.onSubmit();
    }

    render() {
        let schemaOptions = this.props.schemas.map(schema => {
            return <option value={schema.value} key={schema.value}>{schema.label}</option>;
        });
        let yearOptions = this.props.years.map(year => {
            return <option value={year} key={year}>{year}</option>;
        });
        let file = this.props.file
        let selectedFile = file ? <h5><p id="fileName">Selected &ldquo;{file.name}&rdquo;</p></h5> : null;
        let spinner = this.props.validating ? <img src={spinnerImg} alt="spinner" className="spinner" /> : null;
        let errorList;
        if (this.state.errors.length > 0) {
            let items = this.state.errors.map((err, i) => {
                return <li key={"error-" + i}>{err}</li>;
            });
            errorList = <div className="form-errors text-danger">Please correct these error(s):<ul>{items}</ul></div>;
        }

        return (
            <div className="validator-form" id="file-validator-form">
                <form encType="multipart/form-data" onSubmit={this.handleSubmit}>
                    <div className="form-group">
                        <h3>Validate a file</h3>
                        {errorList}
                        <label htmlFor="schema">JSON schema</label>
                        <select className="form-control" id="schema" name="schema" ref="schema" aria-describedby="schemaHelp"
                                onChange={this.handleChange('schema')} value={this.props.schema} required>
                            {schemaOptions}
                        </select>
                        <span id="schemaHelp" className="help-block">The schema of the JSON document to be validated.</span>
                        <label htmlFor="schemaYear">Schema year</label>
                        <select className="form-control" id="schemaYear" name="schemaYear" ref="year" aria-describedby="schemaYearHelp"
                                onChange={this.handleChange('year')} value={this.props.year} required>
                            {yearOptions}
                        </select>
                        <span id="schemaYearHelp" className="help-block">The schema year of the JSON document to be validated.</span>
                    </div>
                    <div className="form-group">
                        <label className="btn btn-default btn-file">
                            Select file <input id="file" type="file" ref="file"
                                               onChange={this.handleChange('file')} style={{display: "none"}} />
                        </label>
                        {selectedFile}
                        <p className="help">
                            Click ‘Validate’ to review the selected file. Please note that validation may take several minutes.
                        </p>
                        <div>
                            <button type="submit" className="btn btn-default" id="validate-file-btn">
                                Validate
                            </button>
                            {spinner}
                        </div>
                    </div>
                </form>
            </div>
        );
    };
}

class TextareaValidatorForm extends Component {
    constructor(props) {
        super(props);
        this.state = {
            errors: []
        };
        this.handleChange = this.handleChange.bind(this);
        this.handleSubmit = this.handleSubmit.bind(this);

        this.formValidator = new TextareaFormValidator();
    }

    handleChange(name) {
        return event => {
            this.props.onChange(name, event.target.value);
        };
    }

    handleSubmit(event) {
        event.preventDefault();

        this.setState({errors: []});

        let schema = ReactDOM.findDOMNode(this.refs.schema).value;
        let year = ReactDOM.findDOMNode(this.refs.year).value;
        let json = ReactDOM.findDOMNode(this.refs.json).value;

        let errors = [];

        if (!this.formValidator.validYear(year)) errors.push(`invalid year: ${year}`);
        if (!this.formValidator.validSchema(schema)) errors.push(`invalid schema: ${schema}`);
        if (!this.formValidator.validJSON(json)) errors.push(`invalid JSON`);

        if (errors.length > 0) {
            this.setState({errors: errors});
            return;
        }

        this.props.onSubmit();
    }

    render() {
        let schemaOptions = this.props.schemas.map(schema => {
            return <option value={schema.value} key={schema.value}>{schema.label}</option>;
        });
        let yearOptions = this.props.years.map(year => {
            return <option value={year} key={year}>{year}</option>;
        });
        let spinner = this.props.validating ? <img src={spinnerImg} alt="spinner" className="spinner" /> : null;
        let errorList;
        if (this.state.errors.length > 0) {
            let items = this.state.errors.map((err, i) => {
                return <li key={"error-" + i}>{err}</li>;
            });
            errorList = <div className="form-errors text-danger">Please correct these error(s):<ul>{items}</ul></div>;
        }

        return (
            <div className="validator-form" id="textarea-validator-form">
                <form onSubmit={this.handleSubmit}>
                    <div className="form-group">
                        <h3>Validate JSON copy/paste</h3>
                        {errorList}
                        <label htmlFor="schema">JSON schema</label>
                        <select className="form-control" id="schema" name="schema" ref="schema" aria-describedby="schemaHelp"
                                onChange={this.handleChange('schema')} value={this.props.schema} required>
                            {schemaOptions}
                        </select>
                        <span id="schemaHelp" className="help-block">The schema of the JSON document to be validated.</span>
                        <label htmlFor="schemaYear">Schema year</label>
                        <select className="form-control" id="schemaYear" name="schemaYear" ref="year" aria-describedby="schemaYearHelp"
                                onChange={this.handleChange('year')} value={this.props.year} required>
                            {yearOptions}
                        </select>
                        <span id="schemaYearHelp" className="help-block">The schema year of the JSON document to be validated.</span>
                    </div>
                    <div className="form-group">
                        <label htmlFor="json">JSON</label>
                        <textarea className="form-control" rows="20" id="json" name="json" ref="json" aria-describedby="jsonHelpBlock"
                                  onChange={this.handleChange('json')} value={this.props.json} required />
                        <span id="jsonHelpBlock" className="help-block">Paste in your JSON here.</span>
                    </div>
                    <div>
                        <p className="help">
                            Click ‘Validate’ to review the JSON you pasted. Please note that validation may take several minutes.
                        </p>
                        <button type="submit" className="btn btn-default" id="validate-file-btn">
                            Validate
                        </button>
                        {spinner}
                    </div>
                </form>
            </div>
        );
    };
}

class ValidationResults extends Component {
    render() {
        let valid = this.props.valid;
        let schema = this.props.schema;
        let classList = `validity${valid ? ' is-valid' : ' is-not-valid'}`;
        
        return (
            <div className="validation-results">
                <div className={classList}>
                    <p className={valid ? 'bg-success' : 'bg-danger'}>This document is <b>{valid ? 'valid' : 'not valid'}</b> {schema} JSON.</p>
                </div>
                <ErrorList errors={this.props.errors} type="error" />
                <ErrorList errors={this.props.warnings} type="warning" />
            </div>
        );
    }
}

class ErrorList extends Component {
    constructor(props) {
        super(props);
        this.state = {showMore: false};
        this.handleToggleClick = this.handleToggleClick.bind(this);
    }

    handleToggleClick() {
        this.setState({showMore: !this.state.showMore});
    }

    textClassName() {
        switch (this.props.type) {
            case 'error':
                return 'text-danger';
            case 'warning':
                return 'text-warning';
            default:
                return '';
        }
    }

    renderList(errors) {
        let items = errors.map((error, i) => {
            return <li key={this.props.type + '-' + i}>{error}</li>;
        });
        return errors.length > 0 ? <ul className={this.textClassName()}>{items}</ul> : null;
    }

    render() {
        const maxShowErrs = 10;
        let errors = this.props.errors;
        let type = this.props.type;
        let dispErrors = errors;
        if (!this.state.showMore) {
            dispErrors = errors.slice(0, maxShowErrs);
        }
        let list = this.renderList(dispErrors);
        let description;
        if (errors.length > 0) {
            let text = `${errors.length} ${type}${errors.length === 1 ? '' : 's'}`;
            if (errors.length > maxShowErrs && dispErrors.length < errors.length) {
                text += `, showing first ${maxShowErrs}`;
            }
            text += '.';
            let toggle;
            if (errors.length > maxShowErrs) {
                toggle = <p className="show-more-link"><a href="#" onClick={this.handleToggleClick}>Show {this.state.showMore ? 'fewer' : 'more'} &hellip;</a></p>;
            }
            let descClass = `description ${this.textClassName()}`;
            description = <div><p className={descClass}>{text}</p>{toggle}</div>;
        }
        
        let classList = `error-list ${type}`;

        return (
            <div className={classList}>
                {description}
                {list}
            </div>
        );
    }
}

export default App;
