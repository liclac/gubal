import dash
from dash.dependencies import Input, Output
import dash_core_components as dcc
import dash_html_components as html
import plotly.graph_objs as go
from flask_sqlalchemy import SQLAlchemy
import pandas as pd

TABLE_CLASSES = 'table table-hover table-sm'

app = dash.Dash()
app.server.config['SQLALCHEMY_DATABASE_URI'] = 'postgres:///gubal'
app.server.config['SQLALCHEMY_TRACK_MODIFICATIONS'] = False
db = SQLAlchemy(app.server)



# Load Bootstrap
app.css.append_css({
    "external_url": "https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css",
})
app.scripts.append_script({
    "external_url": "https://code.jquery.com/jquery-3.2.1.slim.min.js",
})
app.scripts.append_script({
    "external_url": "https://cdnjs.cloudflare.com/ajax/libs/popper.js/1.12.9/umd/popper.min.js",
})
app.scripts.append_script({
    "external_url": "https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js",
})

def build_table(df, **kwargs):
    return html.Table([
        html.Thead([
            html.Tr([html.Th(col) for col in df.columns]),
        ]),
        html.Tbody([
            html.Tr([
                html.Td([
                    df.iloc[i][col]
                ]) for col in df.columns
            ]) for i in range(len(df))
        ]),
    ], **kwargs)

def build_gender_chart(**kwargs):
    df = pd.read_sql('SELECT gender, COUNT(*) FROM characters GROUP BY gender ORDER BY gender DESC', db.engine, index_col='gender')
    return go.Pie(labels=df.index.tolist(), values=df['count'], **kwargs)

def build_race_chart(**kwargs):
    df = pd.read_sql('SELECT race, COUNT(*) FROM characters GROUP BY race ORDER BY race DESC', db.engine, index_col='race')
    return go.Pie(labels=df.index.tolist(), values=df['count'], **kwargs)

def build_race_clan_gender_chart(**kwargs):
    df = pd.read_sql('SELECT race, clan, gender, COUNT(*) FROM characters GROUP BY race, clan, gender ORDER BY race, clan, gender DESC', db.engine, index_col=['race', 'clan', 'gender'])

    # TODO: clean up this mess
    races = df.index.get_level_values('race').unique()
    series = [{"y": [], "text": []} for i in range(4)]
    for race in races:
        race_data = df.loc[race]
        for i, clan_and_gender in enumerate(race_data.index.values):
            series[i]["text"].append(" ".join(clan_and_gender))
            series[i]["y"].append(race_data.loc[clan_and_gender]['count'])
    return [go.Bar(x=races, **s, **kwargs) for s in series]

def build_title_table(**kwargs):
    df = pd.read_sql('SELECT title, COUNT(*) FROM characters INNER JOIN character_titles ON characters.title_id = character_titles.id GROUP BY title ORDER BY count DESC LIMIT 10', db.engine)
    return build_table(df, **kwargs)

def build_top_table(col, limit=10, **kwargs):
    df = pd.read_sql('SELECT {col}, COUNT(*) FROM characters GROUP BY {col} ORDER BY count DESC LIMIT {limit}'.format(col=col, limit=limit), db.engine)
    return build_table(df, **kwargs)

def build_layout():
    return html.Div([
        html.Div([
            dcc.Graph(
                id="gender-chart",
                figure=go.Figure(
                    data=[build_gender_chart(
                        textinfo='label+percent',
                        hole=0.5,
                        marker=go.Marker(
                            colors=['#00CCFF', '#FF0099'],
                            line=go.Line(color='#FFF', width=1),
                        ),
                    )],
                    layout=go.Layout(title="Gender"),
                ),
                className='col-sm-6',
            ),
            dcc.Graph(
                id="race-chart",
                figure=go.Figure(
                    data=[build_race_chart(
                        textinfo='label+percent',
                        hole=0.5,
                        marker=go.Marker(line=go.Line(color='#FFF', width=1)),
                    )],
                    layout=go.Layout(title="Race"),
                ),
                className='col-sm-6',
            ),
        ], className='row', style={'min-height': '500px'}),
        html.Div([
            dcc.Graph(
                id="race-clan-gender-chart",
                figure=go.Figure(
                    data=[*build_race_clan_gender_chart(
                        showlegend=False,
                        hoverinfo='y+text',
                        textposition='auto',
                    )],
                    layout=go.Layout(title="Breakdown", barmode='group'),
                ),
                className='col-sm-12',
            ),
        ], className='row', style={'min-height': '500px'}),
        html.Div([
            html.Div([
                html.H5("Top 10 Titles"),
                build_title_table(className=TABLE_CLASSES),
            ], className='col-sm-4'),
            html.Div([
                html.H5("Top 10 First Names"),
                build_top_table('first_name', className=TABLE_CLASSES),
            ], className='col-sm-4'),
            html.Div([
                html.H5("Top 10 Last Names"),
                build_top_table('last_name', className=TABLE_CLASSES),
            ], className='col-sm-4'),
        ], className='row'),
    ], className='container')

app.layout = build_layout

if __name__ == '__main__':
    app.run_server(debug=True)
