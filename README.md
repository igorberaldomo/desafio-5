Para fazer as requisições:
    Para inicializar o projeto suba o container cdocker composer
    para testar o projeto envie uma requisição do tipo POST exemplo { "cep": "29902555" }
    a requisição será valida somente sem tiver a variavel cep e o valor dessa variavel deve ter tamanho de 8 numeros sem letras e simbolos
    para o endereço http://localhost/8080

Objetivos:

    implementação de um projeto docker composer em que o serviço A irá receber um post de cep o serviço a somente o validará, ele irá chamar o serviço B, que irá realizar a pesquisa do cep para encontrar a cidade e em seguida usará o nome da cidade e retornar as temperaturas.
    
    ambos os serviços irão utilizar o Zipkin e o OTEL para monitorar o fluxo de dados.

Serviço A: se encontra na porta localhost:8080

Requisitos - Serviço A (responsável pelo input):

    O sistema deve receber um input de 8 dígitos via POST, através do schema:  { "cep": "29902555" }
    O sistema deve validar se o input é valido (contem 8 dígitos) e é uma STRING
    Caso seja válido, será encaminhado para o Serviço B via HTTP
    Caso não seja válido, deve retornar:
    Código HTTP: 422
    Mensagem: invalid zipcode


Serviço B: se encontra na porta serviceb:8081 se executado pelo docker-composer
Requisitos - Serviço B (responsável pela orquestração):

    O sistema deve receber um CEP válido de 8 digitos
    O sistema deve realizar a pesquisa do CEP e encontrar o nome da localização, a partir disso, deverá retornar as temperaturas e formata-lás em: Celsius, Fahrenheit, Kelvin juntamente com o nome da localização.
    O sistema deve responder adequadamente nos seguintes cenários:
    Em caso de sucesso:
    Código HTTP: 200
    Response Body: { "city: "São Paulo", "temp_C": 28.5, "temp_F": 28.5, "temp_K": 28.5 }
    Em caso de falha, caso o CEP não seja válido (com formato correto):
    Código HTTP: 422
    Mensagem: invalid zipcode
    ​​​Em caso de falha, caso o CEP não seja encontrado:
    Código HTTP: 404
    Mensagem: can not find zipcode
    Após a implementação dos serviços, adicione a implementação do OTEL + Zipkin:

Implementar tracing distribuído entre Serviço A - Serviço B
Utilizar span para medir o tempo de resposta do serviço de busca de CEP e busca de temperatura

    jaeger http://localhost:16686/ (selecionar Service 'servicea')
    zipkin http://localhost:9411/ (query serviceName=servicea)

Dicas:

    Utilize a API viaCEP (ou similar) para encontrar a localização que deseja consultar a temperatura: https://viacep.com.br/
    Utilize a API WeatherAPI (ou similar) para consultar as temperaturas desejadas: https://www.weatherapi.com/
    Para realizar a conversão de Celsius para Fahrenheit, utilize a seguinte fórmula: F = C * 1,8 + 32
    Para realizar a conversão de Celsius para Kelvin, utilize a seguinte fórmula: K = C + 273
    Sendo F = Fahrenheit
    Sendo C = Celsius
    Sendo K = Kelvin